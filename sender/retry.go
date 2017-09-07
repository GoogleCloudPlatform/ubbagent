// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sender

import (
	"errors"
	"flag"
	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/golang/glog"
	"math"
	"path"
	"sync"
	"time"
)

const (
	persistPrefix = "epqueue"
)

var minRetryDelay = flag.Duration("retrymin", 2*time.Second, "minimum exponential backoff delay")
var maxRetryDelay = flag.Duration("retrymax", 60*time.Second, "maximum exponential backoff delay")

// RetryingSender is a Sender handles sending batches to remote endpoints.
// It buffers reports and retries in the event of a send failure, using exponential backoff between
// retry attempts. Minimum and maximum delays are configurable via the "retrymin" and "retrymax"
// flags.
type RetryingSender struct {
	endpoint    endpoint.Endpoint
	queue       persistence.Queue
	recorder    stats.StatsRecorder
	clock       clock.Clock
	lastAttempt time.Time
	delay       time.Duration
	minDelay    time.Duration
	maxDelay    time.Duration
	add         chan addMsg
	closed      bool
	closeMutex  sync.RWMutex
	wait        sync.WaitGroup
}

type addMsg struct {
	report endpoint.EndpointReport
	result chan error
}

type retryingSend struct {
	rs     *RetryingSender
	report endpoint.EndpointReport
}

// NewRetryingSender creates a new RetryingSender for endpoint, storing state in persistence.
func NewRetryingSender(endpoint endpoint.Endpoint, persistence persistence.Persistence, recorder stats.StatsRecorder) *RetryingSender {
	return newRetryingSender(endpoint, persistence, recorder, clock.NewRealClock(), *minRetryDelay, *maxRetryDelay)
}

func newRetryingSender(endpoint endpoint.Endpoint, persistence persistence.Persistence, recorder stats.StatsRecorder, clock clock.Clock, minDelay, maxDelay time.Duration) *RetryingSender {
	rs := &RetryingSender{
		endpoint: endpoint,
		queue:    persistence.Queue(persistenceName(endpoint.Name())),
		recorder: recorder,
		clock:    clock,
		minDelay: minDelay,
		maxDelay: maxDelay,
		add:      make(chan addMsg, 1),
	}
	rs.wait.Add(1)
	go rs.run()
	return rs
}

func (s *retryingSend) Send() error {
	err := s.rs.send(s.report)
	if err != nil {
		// Record this immediate failure.
		s.rs.recorder.SendFailed(s.BatchId(), s.rs.endpoint.Name())
	}
	return err
}

func (s *retryingSend) BatchId() string {
	return s.report.BatchId()
}

func (s *retryingSend) Handlers() []string {
	return []string{s.rs.endpoint.Name()}
}

func (rs *RetryingSender) Prepare(batch metrics.MetricBatch) (PreparedSend, error) {
	var report endpoint.EndpointReport
	var err error
	if report, err = rs.endpoint.BuildReport(batch); err != nil {
		return nil, err
	}
	return &retryingSend{
		rs:     rs,
		report: report,
	}, nil
}

// Close instructs the RetryingSender to gracefully shutdown. Any reports that have not yet been
// sent will be persisted to disk. Close blocks until the operation has completed.
func (rs *RetryingSender) Close() error {
	rs.closeMutex.Lock()
	if !rs.closed {
		close(rs.add)
		rs.closed = true
	}
	rs.closeMutex.Unlock()
	rs.wait.Wait()
	return nil
}

// send persists batch and queues it for sending to this sender's associated Endpoint. A call to
// send blocks until the report is persisted.
func (rs *RetryingSender) send(report endpoint.EndpointReport) error {
	rs.closeMutex.RLock()
	defer rs.closeMutex.RUnlock()
	if rs.closed {
		return errors.New("RetryingSender: Send called on closed sender")
	}
	msg := addMsg{
		report: report,
		result: make(chan error),
	}
	rs.add <- msg
	return <-msg.result
}

func (rs *RetryingSender) run() {
	// Start with an initial call to maybeSend() to start sending any persisted state.
	rs.maybeSend()
	for {
		var d time.Duration
		if rs.delay == 0 {
			// A delay of 0 means we're not retrying. Effectively disable the retry timer.
			// We'll wakeup when a new report is sent.
			d = time.Duration(math.MaxInt64)
		} else {
			// Compute the time until the next retry attempt.
			// This could be negative, which should result in the timer immediately firing.
			d = rs.delay - rs.clock.Now().Sub(rs.lastAttempt)
		}
		timer := rs.clock.NewTimer(d)
		select {
		case msg, ok := <-rs.add:
			if ok {
				msg.result <- rs.queue.Enqueue(msg.report)
				rs.maybeSend()
			} else {
				// Channel was closed.
				rs.wait.Done()
				return
			}
		case <-timer.GetC():
			rs.maybeSend()
		}
		timer.Stop()
	}
}

// maybeSend retries a pending send if the required time delay has elapsed.
func (rs *RetryingSender) maybeSend() {
	now := rs.clock.Now()
	if now.Before(rs.lastAttempt.Add(time.Duration(rs.delay))) {
		// Not time yet.
		return
	}
	for {
		report := rs.endpoint.EmptyReport()
		err := rs.queue.Peek(report)
		if err == persistence.ErrNotFound {
			break
		}
		if err == nil {
			err = rs.endpoint.Send(report)
		}
		if err != nil {
			// We've encountered a send error. If the error is considered transient, we'll leave it in
			// the queue and retry. Otherwise it's removed from the queue, logged, and recorded as a
			// failure.
			if rs.endpoint.IsTransient(err) {
				// Set next attempt
				rs.lastAttempt = now
				rs.delay = bounded(rs.delay*2, rs.minDelay, rs.maxDelay)
				glog.Warningf("RetryingSender.maybeSend: %+v (will retry)", err)
				break
			} else {
				glog.Errorf("RetryingSender.maybeSend: %+v", err)
				rs.recorder.SendFailed(report.BatchId(), rs.endpoint.Name())
			}
		} else {
			// Send was successful.
			rs.recorder.SendSucceeded(report.BatchId(), rs.endpoint.Name())
		}

		// At this point we've either successfully sent the report or encountered a non-transient error.
		// In either scenario, the report is removed from the queue and the retry delay is reset.
		if err := rs.queue.Dequeue(nil); err != nil {
			glog.Errorf("RetryingSender.maybeSend: removing queue head: %+v", err)
		}
		rs.lastAttempt = now
		rs.delay = 0
	}
}

func bounded(val, min, max time.Duration) time.Duration {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func persistenceName(name string) string {
	return path.Join(persistPrefix, name)
}
