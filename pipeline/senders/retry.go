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

package senders

import (
	"errors"
	"flag"
	"math/rand"
	"path"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/golang/glog"
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
	endpoint    pipeline.Endpoint
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
	entry  queueEntry
	result chan error
}

type queueEntry struct {
	Report   pipeline.EndpointReport
	SendTime time.Time
}

// NewRetryingSender creates a new RetryingSender for endpoint, storing state in persistence.
func NewRetryingSender(endpoint pipeline.Endpoint, persistence persistence.Persistence, recorder stats.Recorder) *RetryingSender {
	return newRetryingSender(endpoint, persistence, recorder, clock.NewClock(), *minRetryDelay, *maxRetryDelay)
}

func newRetryingSender(endpoint pipeline.Endpoint, persistence persistence.Persistence, recorder stats.Recorder, clock clock.Clock, minDelay, maxDelay time.Duration) *RetryingSender {
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
	go rs.run(clock.Now())
	return rs
}

func (rs *RetryingSender) Send(report metrics.StampedMetricReport) error {
	rs.closeMutex.RLock()
	defer rs.closeMutex.RUnlock()
	if rs.closed {
		return errors.New("RetryingSender: Send called on closed sender")
	}

	epr, err := rs.endpoint.BuildReport(report)
	if err != nil {
		rs.recorder.SendFailed(report.Id, rs.endpoint.Name())
		return err
	}

	msg := addMsg{
		entry:  queueEntry{epr, rs.clock.Now()},
		result: make(chan error),
	}
	rs.add <- msg
	err = <-msg.result

	if err != nil {
		// Record this immediate failure.
		rs.recorder.SendFailed(report.Id, rs.endpoint.Name())
	}
	return err
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

func (rs *RetryingSender) run(start time.Time) {
	// Start with an initial call to maybeSend() to start sending any persisted state.
	rs.maybeSend(start)
	for {
		var timer clock.Timer
		if rs.delay == 0 {
			// A delay of 0 means we're not retrying. Disable the retry timer; We'll wakeup when a new
			// report is sent.
			timer = clock.NewStoppedTimer()
		} else {
			// Compute the next retry time, which is the current time + current delay + [0,1000) ms jitter
			now := rs.clock.Now()
			jitter := time.Duration(rand.Int63n(1000)) * time.Millisecond
			nextFire := now.Add(rs.delay - now.Sub(rs.lastAttempt)).Add(jitter)
			timer = rs.clock.NewTimerAt(nextFire)
		}
		select {
		case msg, ok := <-rs.add:
			if ok {
				err := rs.queue.Enqueue(msg.entry)
				if err != nil {
					msg.result <- err
					break
				}

				// Successfully queued the message
				msg.result <- nil
				rs.maybeSend(msg.entry.SendTime)
			} else {
				// Channel was closed.
				rs.wait.Done()
				return
			}
		case now := <-timer.GetC():
			rs.maybeSend(now)
		}
		timer.Stop()
	}
}

// maybeSend retries a pending send if the required time delay has elapsed.
func (rs *RetryingSender) maybeSend(now time.Time) {
	if now.Before(rs.lastAttempt.Add(rs.delay)) {
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
		if senderr := rs.endpoint.Send(entry.Report); senderr != nil {
			// We've encountered a send error. If the error is considered transient and the entry hasn't
			// reached its maximum queue time, we'll leave it in the queue and retry. Otherwise it's
			// removed from the queue, logged, and recorded as a failure.
			expired := rs.clock.Now().Sub(entry.SendTime) > *maxQueueTime
			if !expired && rs.endpoint.IsTransient(senderr) {
				// Set next attempt
				rs.lastAttempt = now
				rs.delay = bounded(rs.delay*2, rs.minDelay, rs.maxDelay)
				glog.Warningf("RetryingSender.maybeSend [%[1]T - transient; will retry]: %[1]s", senderr)
				break
			} else if expired {
				glog.Errorf("RetryingSender.maybeSend [%[1]T - retry expired]: %[1]s", senderr)
				rs.recorder.SendFailed(entry.Report.Id, rs.endpoint.Name())
			} else {
				glog.Errorf("RetryingSender.maybeSend [%[1]T - will NOT retry]: %[1]s", senderr)
				rs.recorder.SendFailed(entry.Report.Id, rs.endpoint.Name())
			}
		} else {
			// Send was successful.
			rs.recorder.SendSucceeded(entry.Report.Id, rs.endpoint.Name())
		}

		// At this point we've either successfully sent the report or encountered a non-transient error.
		// In either scenario, the report is removed from the queue and the retry delay is reset.
		if poperr := rs.queue.Dequeue(nil); poperr != nil {
			// We failed to pop the sent entry off the queue. This isn't recoverable.
			glog.Errorf("RetryingSender.maybeSend: dequeuing from retry queue: " + poperr.Error() + " we've either successfully sent the report or encountered a non-transient error")
			return
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
