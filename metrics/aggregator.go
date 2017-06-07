package metrics

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"reflect"
	"sync"
	"time"
	"ubbagent/clock"
	"ubbagent/persistence"
)

const (
	persistenceName = "aggregator"
)

type addMsg struct {
	report MetricReport
	result chan error
}

type Aggregator struct {
	clock         clock.Clock
	config        Config
	sender        MetricSender
	persistence   persistence.Persistence
	currentBucket *bucket
	pushTimer     *time.Timer
	push          chan chan bool
	add           chan addMsg
	closed        bool
	closeMutex    sync.RWMutex
	wait          sync.WaitGroup
}

// NewAggregator creates a new Aggregator instance and starts its goroutine.
func NewAggregator(conf Config, sender MetricSender, persistence persistence.Persistence) *Aggregator {
	return newAggregator(conf, sender, persistence, clock.NewRealClock())
}

func newAggregator(conf Config, sender MetricSender, persistence persistence.Persistence, clock clock.Clock) *Aggregator {
	agg := &Aggregator{
		config:      conf,
		sender:      sender,
		persistence: persistence,
		clock:       clock,
		push:        make(chan chan bool),
		add:         make(chan addMsg),
	}
	if !agg.loadState() {
		agg.currentBucket = newBucket(clock.Now())
	}
	agg.wait.Add(1)
	go agg.run()
	return agg
}

// AddReport adds a report. Reports are aggregated when possible, during a time period defined by
// the Aggregator's config object. Two reports can be aggregated if they have the same name, contain
// the same labels, and don't contain overlapping time ranges denoted by StartTime and EndTme.
func (h *Aggregator) AddReport(report MetricReport) error {
	if err := report.Validate(h.config); err != nil {
		return err
	}
	h.closeMutex.RLock()
	defer h.closeMutex.RUnlock()
	if h.closed {
		return errors.New("Aggregator: AddReport called on closed aggregator")
	}
	msg := addMsg{
		report: report,
		result: make(chan error, 1),
	}
	h.add <- msg
	return <-msg.result
}

// Close instructs the Aggregator's goroutine to shutdown. Any currently-aggregated metrics will
// be reported to the downstream sender as part of this process.
func (h *Aggregator) Close() error {
	h.closeMutex.Lock()
	defer h.closeMutex.Unlock()
	if !h.closed {
		close(h.add)
		h.closed = true
	}
	return nil
}

// Join blocks until the Aggregator's goroutine has cleaned up and exited.
func (h *Aggregator) Join() {
	h.wait.Wait()
}

func (h *Aggregator) run() {
	running := true
	for running {
		// Set a timer to fire when the current bucket should be pushed.
		remaining := time.Duration(h.config.BufferSeconds)*time.Second -
			h.clock.Now().Sub(h.currentBucket.CreateTime)
		timer := h.clock.NewTimer(remaining)
		select {
		case msg, ok := <-h.add:
			if ok {
				err := h.currentBucket.addReport(msg.report)
				if err == nil {
					// TODO(volkman): possibly rate-limit persistence, or flush to disk at a defined interval.
					// Perhaps a benchmark to determine whether eager persistence is a bottleneck.
					h.persistState()
				}
				msg.result <- err
			} else {
				running = false
			}
		case <-timer.GetC():
			// Time to push the current bucket.
			h.pushBucket()
		}
		timer.Stop()
	}
	h.pushBucket()
	h.wait.Done()
}

func (h *Aggregator) loadState() bool {
	err := h.persistence.Load(persistenceName, &h.currentBucket)
	if err == persistence.ErrNotFound {
		// Didn't find existing state to load.
		return false
	} else if err == nil {
		// We loaded state.
		return true
	}
	// Some other error loading existing state.
	panic(fmt.Sprintf("Error loading aggregator state: %+v", err))
}

func (h *Aggregator) persistState() {
	if err := h.persistence.Store(persistenceName, h.currentBucket); err != nil {
		panic(fmt.Sprintf("Error persisting aggregator state: %+v", err))
	}
}

// pushBucket sends currently-aggregated metrics to the configured MetricSender and resets the
// bucket.
func (h *Aggregator) pushBucket() {
	now := h.clock.Now()
	if h.currentBucket == nil {
		h.currentBucket = newBucket(now)
		return
	}
	var finishedBatch MetricBatch
	for _, namedReports := range h.currentBucket.Reports {
		for _, report := range namedReports {
			finishedBatch = append(finishedBatch, *report.metricReport())
		}
	}
	if len(finishedBatch) > 0 {
		if err := h.sender.Send(finishedBatch); err != nil {
			glog.Errorf("aggregator: error sending finished bucket: %+v", err)
			return
		}
	}
	h.currentBucket = newBucket(now)
	h.persistState()
}

type bucket struct {
	CreateTime time.Time
	Reports    map[string][]*aggregatedReport
}

// aggregatedReport is an extension of MetricReport that supports operations for combining reports.
type aggregatedReport MetricReport

// accept possibly aggregates the given MetricReport into this aggregatedReport. Returns true
// if the report was aggregated, or false if the labels or name don't match. Returns an error if the
// given report could be aggregated (i.e., labels match), but its time range conflicts with the
// existing aggregated range.
func (ar *aggregatedReport) accept(mr MetricReport) (bool, error) {
	if mr.Name != ar.Name || !reflect.DeepEqual(mr.Labels, ar.Labels) {
		return false, nil
	}
	if mr.StartTime.Before(ar.EndTime) {
		return false, errors.New(fmt.Sprintf("Time conflict: %v < %v", mr.StartTime, ar.EndTime))
	}
	// Only one of these values should be non-zero. We rely on prior validation to ensure the proper
	// value (i.e., the one specified in the MetricDefinition) is provided.
	ar.Value.IntValue += mr.Value.IntValue
	ar.Value.DoubleValue += mr.Value.DoubleValue

	// The aggregated end time advances, but the start time remains unchanged.
	ar.EndTime = mr.EndTime
	return true, nil
}

func (ar *aggregatedReport) metricReport() *MetricReport {
	return (*MetricReport)(ar)
}

func newBucket(t time.Time) *bucket {
	return &bucket{
		Reports:    make(map[string][]*aggregatedReport),
		CreateTime: t,
	}
}

func (b *bucket) addReport(mr MetricReport) error {
	for _, ar := range b.Reports[mr.Name] {
		accepted, err := ar.accept(mr)
		if err != nil {
			return err
		}
		if accepted {
			return nil
		}
	}
	b.Reports[mr.Name] = append(b.Reports[mr.Name], (*aggregatedReport)(&mr))
	return nil
}
