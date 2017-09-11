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

package aggregator

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/sender"

	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/golang/glog"
)

const (
	persistenceName = "aggregator"
)

type addMsg struct {
	report metrics.MetricReport
	result chan error
}

// Aggregator is the head of the metrics reporting pipeline. It accepts reports from the reporting
// client, buffers and aggregates for a configured amount of time, and sends them downstream.
// See pipeline.Pipeline.
type Aggregator struct {
	clock         clock.Clock
	config        *config.Metrics
	sender        sender.Sender
	persistence   persistence.Persistence
	recorder      stats.Recorder
	currentBucket *bucket
	pushTimer     *time.Timer
	push          chan chan bool
	add           chan addMsg
	closed        bool
	closeMutex    sync.RWMutex
	wait          sync.WaitGroup
}

// NewAggregator creates a new Aggregator instance and starts its goroutine.
func NewAggregator(conf *config.Metrics, sender sender.Sender, persistence persistence.Persistence, recorder stats.Recorder) *Aggregator {
	return newAggregator(conf, sender, persistence, recorder, clock.NewRealClock())
}

func newAggregator(conf *config.Metrics, sender sender.Sender, persistence persistence.Persistence, recorder stats.Recorder, clock clock.Clock) *Aggregator {
	agg := &Aggregator{
		config:      conf,
		sender:      sender,
		persistence: persistence,
		recorder:    recorder,
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
func (h *Aggregator) AddReport(report metrics.MetricReport) error {
	glog.V(2).Infoln("Aggregator:AddReport()")
	if err := report.Validate(h.config); err != nil {
		return err
	}
	if err := report.AssignBillingName(h.config); err != nil {
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
// be reported to the downstream sender as part of this process. Close blocks until the operation
// has completed.
func (h *Aggregator) Close() error {
	h.closeMutex.Lock()
	if !h.closed {
		close(h.add)
		h.closed = true
	}
	h.closeMutex.Unlock()
	h.wait.Wait()

	// Cascade
	return h.sender.Close()
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
	err := h.persistence.Value(persistenceName).Load(&h.currentBucket)
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
	if err := h.persistence.Value(persistenceName).Store(h.currentBucket); err != nil {
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
	var finishedReports []metrics.MetricReport
	for _, namedReports := range h.currentBucket.Reports {
		for _, report := range namedReports {
			finishedReports = append(finishedReports, *report.metricReport())
		}
	}
	if len(finishedReports) > 0 {
		glog.V(2).Infoln("Aggregator:pushBucket(): sending batch")
		batch, err := metrics.NewMetricBatch(finishedReports)
		if err != nil {
			glog.Errorf("aggregator: error creating batch: %+v", err)
			return
		}
		ps, err := h.sender.Prepare(batch)
		if err != nil {
			glog.Errorf("aggregator: error preparing finished bucket: %+v", err)
			return
		}

		// Register this send with the stats recorder.
		h.recorder.Register(ps)

		if err := ps.Send(); err != nil {
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
type aggregatedReport metrics.MetricReport

// accept possibly aggregates the given MetricReport into this aggregatedReport. Returns true
// if the report was aggregated, or false if the labels or name don't match. Returns an error if the
// given report could be aggregated (i.e., labels match), but its time range conflicts with the
// existing aggregated range.
func (ar *aggregatedReport) accept(mr metrics.MetricReport) (bool, error) {
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

func (ar *aggregatedReport) metricReport() *metrics.MetricReport {
	return (*metrics.MetricReport)(ar)
}

func newBucket(t time.Time) *bucket {
	return &bucket{
		Reports:    make(map[string][]*aggregatedReport),
		CreateTime: t,
	}
}

func (b *bucket) addReport(mr metrics.MetricReport) error {
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
