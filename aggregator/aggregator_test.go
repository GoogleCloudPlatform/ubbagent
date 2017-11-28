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
	"github.com/GoogleCloudPlatform/ubbagent/stats"
)

type mockPreparedSend struct {
	ms *mockSender
	mb metrics.MetricBatch
}

func (ps *mockPreparedSend) Send() error {
	return ps.ms.send(ps.mb)
}

func (ps *mockPreparedSend) BatchId() string {
	return ps.mb.Id
}

func (ps *mockPreparedSend) Handlers() []string {
	return []string{ps.ms.id}
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

func (s *mockSender) Prepare(mb metrics.MetricBatch) (sender.PreparedSend, error) {
	return &mockPreparedSend{ms: s, mb: mb}, nil
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

func (s *mockSender) setBatch(batch metrics.MetricBatch) {
	s.reports.Store(batch)
}

func (s *mockSender) getBatch() metrics.MetricBatch {
	return s.reports.Load().(metrics.MetricBatch)
}

func (s *mockSender) send(mb metrics.MetricBatch) error {
	s.sendMutex.Lock()
	s.setBatch(mb)
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
	ms.setBatch(metrics.MetricBatch{})
	return ms
}

type mockStatsRecorder struct {
	registered []stats.ExpectedSend
}

func (sr *mockStatsRecorder) Register(es stats.ExpectedSend) {
	sr.registered = append(sr.registered, es)
}

func (sr *mockStatsRecorder) SendSucceeded(string, string) {}
func (sr *mockStatsRecorder) SendFailed(string, string)    {}

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

		sender := newMockSender("sender")
		mockClock := clock.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, p, &mockStatsRecorder{}, mockClock)

		if err := a.AddReport(report1); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(report2); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		reports := sender.getBatch().Reports
		if len(reports) > 0 {
			t.Fatalf("Expected no reports, got: %+v", reports)
		}

		// We set a send error on the mock sender to prevent the aggregator from successfully sending
		// its state at Release. A new aggregator created with the same persistence should start with
		// the previous state.
		sender.setSendErr(errors.New("send failure"))
		a.Release()

		// Construct a new aggregator using the same persistence.
		a = newAggregator(metric, bufTime, sender, p, &mockStatsRecorder{}, mockClock)

		sender.doAndWait(t, func() {
			sender.setSendErr(nil)
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{report1, report2}
		reports = sender.getBatch().Reports
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}

		sender.setSendErr(errors.New("send failure"))
		a.Release()

		// Create one more aggregator and ensure it doesn't start with previous state.
		a = newAggregator(metric, bufTime, sender, p, &mockStatsRecorder{}, mockClock)

		if err := a.AddReport(report3); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		sender.doAndWait(t, func() {
			sender.setSendErr(nil)
			mockClock.SetNow(time.Unix(200, 0))
		})

		expected = []metrics.MetricReport{report3}
		reports = sender.getBatch().Reports
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
	a := newAggregator(metric, bufTime, s, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, clock.NewMockClock())
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

	sender := newMockSender("sender")
	mockClock := clock.NewMockClock()

	// Add a report to a zero-state aggregator
	t.Run("Zero state", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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
		sender.doAndWait(t, func() {
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

		reports := sender.getBatch().Reports
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add multiple reports, testing aggregation
	t.Run("Aggregation", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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
		sender.doAndWait(t, func() {
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

		reports := sender.getBatch().Reports
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add two reports with the same name but different labels: no aggregation
	t.Run("Different labels", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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
		sender.doAndWait(t, func() {
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

		reports := sender.getBatch().Reports
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add a report that fails validation: error
	t.Run("Report validation error", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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
		sender.setBatch(metrics.MetricBatch{})
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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

		sender.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(10, 0))
		})

		if len(sender.getBatch().Reports) == 0 {
			t.Fatal("Expected push after timeout, but sender contains no reports")
		}
	})

	// Ensure that a push happens when the aggregator is Released
	t.Run("Push after Release", func(t *testing.T) {
		sender.setBatch(metrics.MetricBatch{})
		sender.setSendErr(nil)
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), &mockStatsRecorder{}, mockClock)

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

		if len(sender.getBatch().Reports) == 0 {
			t.Fatal("Expected push after Release, but sender contains no reports")
		}
	})

	// Ensure that a StatsRecorder is notified about a push
	t.Run("Push registers send", func(t *testing.T) {
		sender.setBatch(metrics.MetricBatch{Id: "testbatch"})
		sender.setSendErr(nil)
		mockClock.SetNow(time.Unix(0, 0))
		sr := &mockStatsRecorder{}
		a := newAggregator(metric, bufTime, sender, persistence.NewMemoryPersistence(), sr, mockClock)

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

		if len(sr.registered) != 1 && sr.registered[0].BatchId() != "testbatch" {
			t.Fatalf("Expected one registered send with id 'testbatch', got: %+v", sr.registered)
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
