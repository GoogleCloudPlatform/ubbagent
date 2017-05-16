package metrics

import (
	"errors"
	"fmt"
	"reflect"
	"time"
	"ubbagent/clock"
)

type addMsg struct {
	report MetricReport
	result chan error
}

type Aggregator struct {
	clock         clock.Clock
	config        Config
	sender        ReportSender
	currentBucket *bucket
	pushTimer     *time.Timer
	quit          chan bool
	push          chan chan bool
	add           chan addMsg
}

// NewAggregator creates a new Aggregator instance and starts its goroutine.
func NewAggregator(conf Config, sender ReportSender) *Aggregator {
	agg := &Aggregator{
		config: conf,
		sender: sender,
		clock:  clock.NewRealClock(),
		quit:   make(chan bool, 1),
		push:   make(chan chan bool),
		add:    make(chan addMsg),
	}
	go agg.run()
	return agg
}

// AddReport adds a report. Reports are aggregated when possible, during a time period defined by
// the Aggregator's config object. Two reports can be aggregated if they have the same name, contain
// the same labels, and don't contain overlapping time ranges denoted by StartTime and EndTme.
//
// TODO(volkman): There's still a race condition in which AddReport is called after Close, and after
// the goroutine has exited.
func (h *Aggregator) AddReport(report MetricReport) error {
	if err := report.Validate(h.config); err != nil {
		return err
	}
	msg := addMsg{
		report: report,
		result: make(chan error, 1),
	}
	h.add <- msg
	return <-msg.result
}

// Push notifies the aggregator that it should attempt to push its bucket downstream if the
// appropriate amount of time has elapsed. A call to Push blocks until the Aggregator's goroutine
// has processed the request. If the bucket's buffering time has not yet elapsed, the request
// results in a no-op.
//
// TODO(volkman): this method is really only used for testing. Replace in a future change with a
// mock timer.
func (h *Aggregator) Push() {
	resp := make(chan bool, 1)
	h.push <- resp
	<-resp
}

// Close instructs the Aggregator's goroutine to shutdown.
func (h *Aggregator) Close() error {
	// TODO(volkman): Close() might need to block until the goroutine exits to allow for graceful
	// cleanup.
	// TODO(volkman): Remove the quit channel and instead simply close the add channel.
	h.quit <- true
	return nil
}

func (h *Aggregator) run() {
	running := true
	for running {
		h.pushBucket()
		// Set a timer to fire when the current bucket should be pushed.
		remaining := time.Duration(h.config.BufferSeconds)*time.Second -
			h.clock.Now().Sub(h.currentBucket.createTime)
		if remaining < 1*time.Second {
			remaining = 1 * time.Second
		}
		timer := time.NewTimer(remaining)
		select {
		case msg := <-h.add:
			err := h.currentBucket.addReport(msg.report)
			if err == nil {
				h.persistState()
			}
			msg.result <- err
		case <-h.quit:
			running = false
		case resp := <-h.push:
			// the Push() method was called, which means the caller is waiting for the push to occur. Call
			// pushBucket() prior to responding.
			h.pushBucket()
			resp <- true
		case <-timer.C: // timeout
		}
		timer.Stop()
	}
	// TODO(volkman): push the current bucket prior to the exiting.
}

func (h *Aggregator) persistState() {
	// TODO(volkman): implement.
}

func (h *Aggregator) pushBucket() {
	now := h.clock.Now()
	if h.currentBucket == nil {
		h.currentBucket = newBucket(now)
		return
	}

	deadline := h.currentBucket.createTime.Add(time.Duration(h.config.BufferSeconds) * time.Second)
	if !now.Before(deadline) { // !Before == After or Equal
		finishedReports := []MetricReport{}
		for _, namedReports := range h.currentBucket.reports {
			for _, report := range namedReports {
				finishedReports = append(finishedReports, *report.metricReport())
			}
		}
		if len(finishedReports) > 0 {
			h.sender.Send(finishedReports)
		}
		h.currentBucket = newBucket(now)
		h.persistState()
	}
}

type bucket struct {
	createTime time.Time
	reports    map[string][]*aggregatedReport
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
		reports:    make(map[string][]*aggregatedReport),
		createTime: t,
	}
}

func (b *bucket) addReport(mr MetricReport) error {
	for _, ar := range b.reports[mr.Name] {
		accepted, err := ar.accept(mr)
		if err != nil {
			return err
		}
		if accepted {
			return nil
		}
	}
	b.reports[mr.Name] = append(b.reports[mr.Name], (*aggregatedReport)(&mr))
	return nil
}
