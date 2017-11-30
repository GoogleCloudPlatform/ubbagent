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
	"encoding/json"
	"errors"
	"flag"
	"math"
	"path"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
)

const (
	persistPrefix = "epqueue"
)

var minRetryDelay = flag.Duration("min_retry_delay", 2*time.Second, "minimum exponential backoff delay")
var maxRetryDelay = flag.Duration("max_retry_delay", 60*time.Second, "maximum exponential backoff delay")
var maxQueueTime = flag.Duration("max_queue_time", 3*time.Hour, "maximum amount of time to keep an entry in the retry queue")

// RetryingSender is a Sender handles sending reports to remote endpoints.
// It buffers reports and retries in the event of a send failure, using exponential backoff between
// retry attempts. Minimum and maximum delays are configurable via the "retrymin" and "retrymax"
// flags.
type RetryingSender struct {
	endpoint    endpoint.Endpoint
	queue       persistence.Queue
	recorder    stats.Recorder
	clock       clock.Clock
	lastAttempt time.Time
	delay       time.Duration
	minDelay    time.Duration
	maxDelay    time.Duration
	add         chan addMsg
	closed      bool
	closeMutex  sync.RWMutex
	wait        sync.WaitGroup
	tracker     pipeline.UsageTracker
}

type addMsg struct {
	reports []endpoint.EndpointReport
	result  chan error
}

type retryingSend struct {
	rs      *RetryingSender
	reports []endpoint.EndpointReport
}

type queueEntry struct {
	SendTime time.Time
	Report   json.RawMessage
}

func newQueueEntry(report endpoint.EndpointReport, sendTime time.Time) (*queueEntry, error) {
	bytes, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	return &queueEntry{Report: json.RawMessage(bytes), SendTime: sendTime}, nil
}

// NewRetryingSender creates a new RetryingSender for endpoint, storing state in persistence.
func NewRetryingSender(endpoint endpoint.Endpoint, persistence persistence.Persistence, recorder stats.Recorder) *RetryingSender {
	return newRetryingSender(endpoint, persistence, recorder, clock.NewRealClock(), *minRetryDelay, *maxRetryDelay)
}

func newRetryingSender(endpoint endpoint.Endpoint, persistence persistence.Persistence, recorder stats.Recorder, clock clock.Clock, minDelay, maxDelay time.Duration) *RetryingSender {
	rs := &RetryingSender{
		endpoint: endpoint,
		queue:    persistence.Queue(persistenceName(endpoint.Name())),
		recorder: recorder,
		clock:    clock,
		minDelay: minDelay,
		maxDelay: maxDelay,
		add:      make(chan addMsg, 1),
	}
	endpoint.Use()
	rs.wait.Add(1)
	go rs.run()
	return rs
}

func (s *retryingSend) Send() error {
	err := s.rs.send(s.reports)
	if err != nil {
		// Record this immediate failure.
		for _, r := range s.reports {
			s.rs.recorder.SendFailed(r.Id(), s.rs.endpoint.Name())
		}
	}
	return err
}

func (rs *RetryingSender) Prepare(reports ...metrics.StampedMetricReport) (PreparedSend, error) {
	var ers []endpoint.EndpointReport
	for _, r := range reports {
		er, err := rs.endpoint.BuildReport(r)
		if err != nil {
			return nil, err
		}
		ers = append(ers, er)
	}
	return &retryingSend{
		rs:      rs,
		reports: ers,
	}, nil
}

func (rs *RetryingSender) Endpoints() []string {
	return []string{rs.endpoint.Name()}
}

// Use increments the RetryingSender's usage count.
// See pipeline.Component.Use.
func (rs *RetryingSender) Use() {
	rs.tracker.Use()
}

// Release decrements the RetryingSender's usage count. If it reaches 0, Release instructs the
// RetryingSender to gracefully shutdown. Any reports that have not yet been
// sent will be persisted to disk, and the wrapped Endpoint will be released. Release blocks until
// the operation has completed.
// See pipeline.Component.Release.
func (rs *RetryingSender) Release() error {
	return rs.tracker.Release(func() error {
		rs.closeMutex.Lock()
		if !rs.closed {
			close(rs.add)
			rs.closed = true
		}
		rs.closeMutex.Unlock()
		rs.wait.Wait()
		return rs.endpoint.Release()
	})
}

// send persists the given reports and queues them for sending to this sender's associated Endpoint.
// A call to send blocks until the report is persisted.
func (rs *RetryingSender) send(reports []endpoint.EndpointReport) error {
	rs.closeMutex.RLock()
	defer rs.closeMutex.RUnlock()
	if rs.closed {
		return errors.New("RetryingSender: Send called on closed sender")
	}
	msg := addMsg{
		reports: reports,
		result:  make(chan error),
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
				now := rs.clock.Now()
				var merr *multierror.Error
				for _, report := range msg.reports {
					entry, err := newQueueEntry(report, now)
					if err != nil {
						merr = multierror.Append(merr, err)
						continue
					}

					err = rs.queue.Enqueue(entry)
					if err != nil {
						merr = multierror.Append(merr, err)
					}
				}
				msg.result <- merr.ErrorOrNil()
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
		entry := &queueEntry{}
		if loaderr := rs.queue.Peek(entry); loaderr == persistence.ErrNotFound {
			break
		} else if loaderr != nil {
			// We failed to load from the persistent queue. This isn't recoverable.
			panic("RetryingSender.maybeSend: loading from retry queue: " + loaderr.Error())
		}
		report := rs.endpoint.EmptyReport()
		if loaderr := json.Unmarshal(entry.Report, report); loaderr != nil {
			// Failed to unmarshal this report. Log an error and attempt to pop it off the queue down below.
			glog.Errorf("Failed to unmarshal report; removing from queue: %+v", loaderr)
		} else {
			if senderr := rs.endpoint.Send(report); senderr != nil {
				// We've encountered a send error. If the error is considered transient and the entry hasn't
				// reached its maximum queue time, we'll leave it in the queue and retry. Otherwise it's
				// removed from the queue, logged, and recorded as a failure.
				expired := rs.clock.Now().Sub(entry.SendTime) > *maxQueueTime
				if !expired && rs.endpoint.IsTransient(senderr) {
					// Set next attempt
					rs.lastAttempt = now
					rs.delay = bounded(rs.delay*2, rs.minDelay, rs.maxDelay)
					glog.Warningf("RetryingSender.maybeSend: %+v (will retry)", senderr)
					break
				} else if expired {
					glog.Errorf("RetryingSender.maybeSend: %+v (retry expired)", senderr)
					rs.recorder.SendFailed(report.Id(), rs.endpoint.Name())
				} else {
					glog.Errorf("RetryingSender.maybeSend: %+v", senderr)
					rs.recorder.SendFailed(report.Id(), rs.endpoint.Name())
				}
			} else {
				// Send was successful.
				rs.recorder.SendSucceeded(report.Id(), rs.endpoint.Name())
			}
		}

		// At this point we've either successfully sent the report or encountered a non-transient error.
		// In either scenario, the report is removed from the queue and the retry delay is reset.
		if poperr := rs.queue.Dequeue(nil); poperr != nil {
			// We failed to pop the sent entry off the queue. This isn't recoverable.
			panic("RetryingSender.maybeSend: dequeuing from retry queue: " + poperr.Error())
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
