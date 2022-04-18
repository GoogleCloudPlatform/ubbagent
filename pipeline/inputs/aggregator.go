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

package inputs

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/util"
	"github.com/golang/glog"
)

const (
	persistencePrefix = "aggregator/"
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
	metric        metrics.Definition
	bufferTime    time.Duration
	input         pipeline.Input
	persistence   persistence.Persistence
	currentBucket *bucket
	pushTimer     *time.Timer
	push          chan chan bool
	add           chan addMsg
	closed        bool
	closeMutex    sync.RWMutex
	wait          sync.WaitGroup
	tracker       pipeline.UsageTracker
}

// NewAggregator creates a new Aggregator instance and starts its goroutine.
func NewAggregator(metric metrics.Definition, bufferTime time.Duration, input pipeline.Input, persistence persistence.Persistence) *Aggregator {
	return newAggregator(metric, bufferTime, input, persistence, clock.NewClock())
}

func newAggregator(metric metrics.Definition, bufferTime time.Duration, input pipeline.Input, persistence persistence.Persistence, clock clock.Clock) *Aggregator {
	agg := &Aggregator{
		metric:      metric,
		bufferTime:  bufferTime,
		input:       input,
		persistence: persistence,
		clock:       clock,
		push:        make(chan chan bool),
		add:         make(chan addMsg),
	}
	if !agg.loadState() {
		agg.currentBucket = newBucket(clock.Now())
	}
	input.Use()
	agg.wait.Add(1)
	go agg.run()
	return agg
}

// AddReport adds a report. Reports are aggregated when possible, during a time period defined by
// the Aggregator's config object. Two reports can be aggregated if they have the same name, contain
// the same labels, and don't contain overlapping time ranges denoted by StartTime and EndTme.
func (h *Aggregator) AddReport(report metrics.MetricReport) error {
	glog.V(2).Infof("aggregator: received report: %v", report.Name)
	if err := report.Validate(h.metric); err != nil {
		return err
	}
	h.closeMutex.RLock()
	defer h.closeMutex.RUnlock()
	if h.closed {
		return errors.New("aggregator: AddReport called on closed aggregator")
	}
	msg := addMsg{
		report: report,
		result: make(chan error, 1),
	}
	h.add <- msg
	return <-msg.result
}

// Use increments the Aggregator's usage count.
// See pipeline.Component.Use.
func (h *Aggregator) Use() {
	h.tracker.Use()
}

// Release decrements the Aggregator's usage count. If it reaches 0, Release instructs the
// Aggregator's goroutine to shutdown. Any currently-aggregated metrics will
// be reported to the downstream sender as part of this process. Release blocks until the operation
// has completed.
// See pipeline.Component.Release.
func (h *Aggregator) Release() error {
	return h.tracker.Release(func() error {
		h.closeMutex.Lock()
		if !h.closed {
			close(h.add)
			h.closed = true
		}
		h.closeMutex.Unlock()
		h.wait.Wait()

		// Cascade
		return h.input.Release()
	})
}

func (h *Aggregator) run() {
	running := true
	for running {
		// Set a timer to fire when the current bucket should be pushed.
		now := h.clock.Now()
		nextFire := now.Add(h.bufferTime - now.Sub(h.currentBucket.CreateTime))
		timer := h.clock.NewTimerAt(nextFire)
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
		case now := <-timer.GetC():
			// Time to push the current bucket.
			h.pushBucket(now)
		}
		timer.Stop()
	}
	h.pushBucket(h.clock.Now())
	h.wait.Done()
}

func (h *Aggregator) loadState() bool {
	err := h.persistence.Value(h.persistenceName()).Load(&h.currentBucket)
	if err == persistence.ErrNotFound {
		// Didn't find existing state to load.
		return false
	} else if err == nil {
		// We loaded state.
		return true
	}
	// Some other error loading existing state.
	panic(fmt.Sprintf("error loading aggregator state: %+v", err))
}

func (h *Aggregator) persistState() {
	// TODO(volkman): always persist a metric's previous end time, even if no bucket is persisted,
	// so that the start time of the next report after a restart is validated.
	if err := h.persistence.Value(h.persistenceName()).Store(h.currentBucket); err != nil {
		panic(fmt.Sprintf("error persisting aggregator state: %+v", err))
	}
}

// pushBucket sends currently-aggregated metrics to the configured MetricSender and resets the
// bucket.
func (h *Aggregator) pushBucket(now time.Time) {
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
		if len(finishedReports) == 1 {
			glog.V(2).Infoln("aggregator: sending 1 report")
		} else {
			glog.V(2).Infof("aggregator: sending %v reports", len(finishedReports))
		}
		for _, r := range finishedReports {
			err := h.input.AddReport(r)
			if err != nil {
				glog.Errorf("aggregator: error sending report: %+v", err)
				continue
			}
		}
	}
	h.currentBucket = newBucket(now)
	h.persistState()
}

func (h *Aggregator) persistenceName() string {
	return persistencePrefix + h.metric.Name
}

type bucket struct {
	CreateTime time.Time
	Reports    map[string][]*aggregatedReport
}

// aggregatedReport is an extension of MetricReport that supports operations for combining reports.
type aggregatedReport metrics.MetricReport

// accept possibly aggregates the given MetricReport into this aggregatedReport. Returns true
// if the report was aggregated, or false if the labels or name don't match.
func (ar *aggregatedReport) accept(mr metrics.MetricReport) (bool, error) {
	if mr.Name != ar.Name || !reflect.DeepEqual(mr.Labels, ar.Labels) {
		return false, nil
	}

	// Only one of these values should be non-nil. We rely on prior validation to ensure the proper
	// value (i.e., the one specified in the metrics.Definition) is provided.
	if mr.Value.Int64Value != nil {
		if ar.Value.Int64Value == nil {
			ar.Value.Int64Value = util.NewInt64(0)
		}
		*ar.Value.Int64Value += *mr.Value.Int64Value
	} else if mr.Value.DoubleValue != nil {
		if ar.Value.DoubleValue == nil {
			ar.Value.DoubleValue = util.NewFloat64(0)
		}
		*ar.Value.DoubleValue += *mr.Value.DoubleValue
	}

	// Expand the aggregated start time if the given MetricReport has ealier start time.
	if mr.StartTime.Before(ar.StartTime) {
		ar.StartTime = mr.StartTime
	}
	// Expand the aggregated end time if the given MetricReport has later end time.
	if mr.EndTime.After(ar.StartTime) {
		ar.EndTime = mr.EndTime
	}
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
