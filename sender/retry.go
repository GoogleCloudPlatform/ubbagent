package sender

import (
	"errors"
	"flag"
	"github.com/golang/glog"
	"math"
	"path"
	"reflect"
	"sync"
	"time"
	"ubbagent/clock"
	"ubbagent/persistence"
	"ubbagent/metrics"
	"ubbagent/endpoint"
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
	persistence persistence.Persistence
	queue       []endpoint.EndpointReport
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
func NewRetryingSender(endpoint endpoint.Endpoint, persistence persistence.Persistence) *RetryingSender {
	return newRetryingSender(endpoint, persistence, clock.NewRealClock(), *minRetryDelay, *maxRetryDelay)
}

func newRetryingSender(endpoint endpoint.Endpoint, persistence persistence.Persistence, clock clock.Clock, minDelay, maxDelay time.Duration) *RetryingSender {
	rs := &RetryingSender{
		endpoint:    endpoint,
		persistence: persistence,
		clock:       clock,
		minDelay:    minDelay,
		maxDelay:    maxDelay,
		add:         make(chan addMsg, 1),
	}
	rs.loadQueue()
	rs.wait.Add(1)
	go rs.run()
	return rs
}

func (s *retryingSend) Send() error {
	return s.rs.send(s.report)
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
	for {
		if len(rs.queue) > 0 && rs.delay == 0 {
			// This condition might happen when the RetrySender has just been created but has loaded a
			// previous queue. The queue is not empty, so we prime the retry delay with the defined min.
			rs.maybeSend()
		}
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
				rs.queue = append(rs.queue, msg.report)
				msg.result <- rs.storeQueue()
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
	if len(rs.queue) == 0 {
		// Nothing to do.
		return
	}
	if now.Before(rs.lastAttempt.Add(time.Duration(rs.delay))) {
		// Not time yet.
		return
	}
	for len(rs.queue) > 0 {
		report := rs.queue[0]
		if err := rs.endpoint.Send(report); err != nil {
			// Set next attempt
			rs.lastAttempt = now
			rs.delay = bounded(rs.delay*2, rs.minDelay, rs.maxDelay)
			glog.Errorf("RetryingSender.maybeSend: %+v", err)
			break
		}
		// We've successfully sent the first report, so remove it from the queue and reset the delay.
		rs.queue[0] = nil // zero-out first item so the slice's underlying array doesn't prevent GC.
		rs.queue = rs.queue[1:]
		rs.lastAttempt = now
		rs.delay = 0
		rs.storeQueue()
	}
}

func (rs *RetryingSender) loadQueue() {
	reportType := reflect.TypeOf(rs.endpoint.EmptyReport())
	loadedQueue := reflect.MakeSlice(reflect.SliceOf(reportType), 0, 0).Interface()
	if err := rs.persistence.Load(persistenceName(rs.endpoint.Name()), &loadedQueue); err != nil && err != persistence.ErrNotFound {
		glog.Errorf("RetryingSender.loadQueue: %+v", err)
		return
	}
	genericQueue := reflect.ValueOf(loadedQueue)
	reportQueue := make([]endpoint.EndpointReport, genericQueue.Len())
	for i := 0; i < genericQueue.Len(); i++ {
		reportQueue[i] = genericQueue.Index(i).Interface().(endpoint.EndpointReport)
	}
	rs.queue = reportQueue
}

func (rs *RetryingSender) storeQueue() error {
	err := rs.persistence.Store(persistenceName(rs.endpoint.Name()), rs.queue)
	if err != nil {
		glog.Errorf("RetryingSender.storeQueue: %+v", err)
	}
	return err
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
