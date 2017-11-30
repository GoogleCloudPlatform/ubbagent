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
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/sender"
)

type mockPreparedSend struct {
	ms      *mockSender
	reports []metrics.StampedMetricReport
}

func (ps *mockPreparedSend) Send() error {
	return ps.ms.send(ps.reports...)
}

type mockSender struct {
	id        string
	reports   atomic.Value
	sendMutex sync.Mutex
	errMutex  sync.Mutex
	sendErr   error
	waitChan  chan bool
	released  bool
}

func (s *mockSender) Prepare(reports ...metrics.StampedMetricReport) (sender.PreparedSend, error) {
	return &mockPreparedSend{ms: s, reports: reports}, nil
}

func (s *mockSender) Endpoints() (empty []string) {
	return
}

func (s *mockSender) Use() {}

func (s *mockSender) Release() error {
	s.released = true
	return nil
}

func (s *mockSender) setSendErr(err error) {
	s.errMutex.Lock()
	s.sendErr = err
	s.errMutex.Unlock()
}

func (s *mockSender) getSendErr() (err error) {
	s.errMutex.Lock()
	err = s.sendErr
	s.errMutex.Unlock()
	return
}

func (s *mockSender) setReports(reports []metrics.MetricReport) {
	s.reports.Store(reports)
}

func (s *mockSender) getReports() []metrics.MetricReport {
	return s.reports.Load().([]metrics.MetricReport)
}

func (s *mockSender) clearReports() {
	s.setReports([]metrics.MetricReport{})
}

func (s *mockSender) send(reports ...metrics.StampedMetricReport) error {
	s.sendMutex.Lock()
	var r []metrics.MetricReport
	for _, sr := range reports {
		r = append(r, sr.MetricReport)
	}
	s.setReports(r)
	if s.waitChan != nil {
		s.waitChan <- true
		s.waitChan = nil
	}
	s.sendMutex.Unlock()
	return s.getSendErr()
}

func (s *mockSender) doAndWait(t *testing.T, f func()) {
	waitChan := make(chan bool, 1)
	s.sendMutex.Lock()
	s.waitChan = waitChan
	f()
	s.sendMutex.Unlock()
	select {
	case <-waitChan:
	case <-time.After(5 * time.Second):
		t.Fatal("doAndWait: nothing happened after 5 seconds")
	}
}

func newMockSender(id string) *mockSender {
	ms := &mockSender{id: id}
	ms.clearReports()
	return ms
}

func TestNewAggregator(t *testing.T) {
	t.Run("Load previous state", func(t *testing.T) {
		// Ensures that a new aggregator loads previous state
		p := persistence.NewMemoryPersistence()

		metric := config.MetricDefinition{
			Name: "int-metric",
			Type: "int",
		}
		bufTime := 10 * time.Second

		report1 := metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
			Labels: map[string]string{
				"key": "value1",
			},
		}

		report2 := metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
			Value: metrics.MetricValue{
				IntValue: 333,
			},
			Labels: map[string]string{
				"key": "value2",
			},
		}

		report3 := metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(100, 0),
			EndTime:   time.Unix(110, 0),
			Value: metrics.MetricValue{
				IntValue: 555,
			},
			Labels: map[string]string{
				"key": "value3",
			},
		}

		ms := newMockSender("sender")
		mockClock := clock.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, p, mockClock)

		if err := a.AddReport(report1); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(report2); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		reports := ms.getReports()
		if len(reports) > 0 {
			t.Fatalf("Expected no reports, got: %+v", reports)
		}

		// We set a send error on the mock sender to prevent the aggregator from successfully sending
		// its state at Release. A new aggregator created with the same persistence should start with
		// the previous state.
		ms.setSendErr(errors.New("send failure"))
		a.Release()

		// Construct a new aggregator using the same persistence.
		a = newAggregator(metric, bufTime, ms, p, mockClock)

		ms.doAndWait(t, func() {
			ms.setSendErr(nil)
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{report1, report2}
		reports = ms.getReports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}

		ms.setSendErr(errors.New("send failure"))
		a.Release()

		// Create one more aggregator and ensure it doesn't start with previous state.
		a = newAggregator(metric, bufTime, ms, p, mockClock)

		if err := a.AddReport(report3); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		ms.doAndWait(t, func() {
			ms.setSendErr(nil)
			mockClock.SetNow(time.Unix(200, 0))
		})

		expected = []metrics.MetricReport{report3}
		reports = ms.getReports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
		a.Release()
	})
}

func TestAggregator_Use(t *testing.T) {
	s := newMockSender("sender")
	metric := config.MetricDefinition{}
	bufTime := 10 * time.Second

	// Test multiple usages of the Aggregator.
	a := newAggregator(metric, bufTime, s, persistence.NewMemoryPersistence(), clock.NewMockClock())
	a.Use()
	a.Use()

	a.Release() // Usage count should still be 1.
	if s.released {
		t.Fatal("sender.released expected to be false")
	}

	a.Release() // Usage count should be 0; sender should be released.
	if !s.released {
		t.Fatal("sender.released expected to be true")
	}
}

func TestAggregator_AddReport(t *testing.T) {
	metric := config.MetricDefinition{
		Name: "int-metric",
		Type: "int",
	}
	bufTime := 1 * time.Second

	ms := newMockSender("sender")
	mockClock := clock.NewMockClock()

	// Add a report to a zero-state aggregator
	t.Run("Zero state", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		ms.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					IntValue: 10,
				},
			},
		}

		reports := ms.getReports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add multiple reports, testing aggregation
	t.Run("Aggregation", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: metrics.MetricValue{
				IntValue: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		ms.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(3, 0),
				Value: metrics.MetricValue{
					IntValue: 15,
				},
			},
		}

		reports := ms.getReports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add two reports with the same name but different labels: no aggregation
	t.Run("Different labels", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Labels: map[string]string{
				"key1": "value1",
			},
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Labels: map[string]string{
				"key1": "value2",
			},
			Value: metrics.MetricValue{
				IntValue: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		ms.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Labels: map[string]string{
					"key1": "value1",
				},
				Value: metrics.MetricValue{
					IntValue: 10,
				},
			},
			{
				Name:      "int-metric",
				StartTime: time.Unix(2, 0),
				EndTime:   time.Unix(3, 0),
				Labels: map[string]string{
					"key1": "value2",
				},
				Value: metrics.MetricValue{
					IntValue: 5,
				},
			},
		}

		reports := ms.getReports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add a report that fails validation: error
	t.Run("Report validation error", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(10, 0), // StartTime > EndTime -> error
			EndTime:   time.Unix(1, 0),
			Labels: map[string]string{
				"key1": "value1",
			},
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err == nil || !strings.Contains(err.Error(), "StartTime > EndTime") {
			t.Fatalf("Expected error containing \"StartTime > EndTime\", got: %+v", err.Error())
		}
	})

	// Add a report with a start time less than the last end time: error
	t.Run("Time conflict", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(2, 0),
			Value: metrics.MetricValue{
				IntValue: 5,
			},
		}); err == nil || !strings.Contains(err.Error(), "Time conflict") {
			t.Fatalf("Expected error containing \"Time conflict\", got: %+v", err)
		}
	})

	// Ensure that the push occurs automatically after a timeout
	t.Run("Push after timeout", func(t *testing.T) {
		ms.clearReports()
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		ms.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(10, 0))
		})

		if len(ms.getReports()) == 0 {
			t.Fatal("Expected push after timeout, but sender contains no reports")
		}
	})

	// Ensure that a push happens when the aggregator is Released
	t.Run("Push after Release", func(t *testing.T) {
		ms.clearReports()
		ms.setSendErr(nil)
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, ms, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		a.Release()

		if len(ms.getReports()) == 0 {
			t.Fatal("Expected push after Release, but sender contains no reports")
		}
	})
}

func equalUnordered(a, b []metrics.MetricReport) bool {
	if len(a) != len(b) {
		return false
	}
	used := make(map[int]bool)
	count := 0
	for _, iobj := range a {
		for j, jobj := range b {
			if used[j] {
				continue
			}
			if reflect.DeepEqual(iobj, jobj) {
				used[j] = true
				count += 1
				break
			}
		}
	}
	return count == len(a)
}
