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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

func TestNewAggregator(t *testing.T) {
	t.Run("Load previous state", func(t *testing.T) {
		// Ensures that a new aggregator loads previous state
		p := persistence.NewMemoryPersistence()

		metric := metrics.Definition{
			Name: "int-metric",
			Type: "int",
		}
		bufTime := 10 * time.Second

		report1 := metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
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
				Int64Value: 333,
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
				Int64Value: 555,
			},
			Labels: map[string]string{
				"key": "value3",
			},
		}

		mi := testlib.NewMockInput()
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(metric, bufTime, mi, p, mockClock)

		if err := a.AddReport(report1); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(report2); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		reports := mi.Reports()
		if len(reports) > 0 {
			t.Fatalf("Expected no reports, got: %+v", reports)
		}

		// Use a new MockClock so that manipulating it doesn't trigger the first aggregator, which is
		// still running.
		mockClock = testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))

		// Construct a new aggregator using the same persistence.
		a = newAggregator(metric, bufTime, mi, p, mockClock)

		// Release the aggregator so that it flushes all of its current reports.
		mi.DoAndWait(t, 2, func() {
			a.Release()
		})

		expected := []metrics.MetricReport{report1, report2}
		reports = mi.Reports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}

		mockClock = testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))

		// Create one more aggregator and ensure it doesn't start with previous state.
		a = newAggregator(metric, bufTime, mi, p, mockClock)

		if err := a.AddReport(report3); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		mi.DoAndWait(t, 3, func() {
			mockClock.SetNow(time.Unix(200, 0))
		})

		expected = []metrics.MetricReport{report3}
		reports = mi.Reports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})
}

func TestAggregator_Use(t *testing.T) {
	mi := testlib.NewMockInput()
	metric := metrics.Definition{}
	bufTime := 10 * time.Second

	// Test multiple usages of the Aggregator.
	a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), testlib.NewMockClock())
	a.Use()
	a.Use()

	a.Release() // Usage count should still be 1.
	if mi.Released {
		t.Fatal("sender.released expected to be false")
	}

	a.Release() // Usage count should be 0; sender should be released.
	if !mi.Released {
		t.Fatal("sender.released expected to be true")
	}
}

func TestAggregator_AddReport(t *testing.T) {
	metric := metrics.Definition{
		Name: "int-metric",
		Type: "int",
	}
	bufTime := 1 * time.Second

	// Add a report to a zero-state aggregator
	t.Run("Zero state", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mi.DoAndWait(t, 1, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					Int64Value: 10,
				},
			},
		}

		reports := mi.Reports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add multiple reports, testing aggregation
	t.Run("Aggregation", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: metrics.MetricValue{
				Int64Value: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mi.DoAndWait(t, 1, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []metrics.MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(3, 0),
				Value: metrics.MetricValue{
					Int64Value: 15,
				},
			},
		}

		reports := mi.Reports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add two reports with the same name but different labels: no aggregation
	t.Run("Different labels", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Labels: map[string]string{
				"key1": "value1",
			},
			Value: metrics.MetricValue{
				Int64Value: 10,
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
				Int64Value: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mi.DoAndWait(t, 2, func() {
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
					Int64Value: 10,
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
					Int64Value: 5,
				},
			},
		}

		reports := mi.Reports()
		if !equalUnordered(reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, reports)
		}
	})

	// Add a report that fails validation: error
	t.Run("Report validation error", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(10, 0), // StartTime > EndTime -> error
			EndTime:   time.Unix(1, 0),
			Labels: map[string]string{
				"key1": "value1",
			},
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err == nil || !strings.Contains(err.Error(), "StartTime > EndTime") {
			t.Fatalf("Expected error containing \"StartTime > EndTime\", got: %+v", err.Error())
		}
	})

	// Add a report with a start time less than the last end time: error
	t.Run("Time conflict", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(2, 0),
			Value: metrics.MetricValue{
				Int64Value: 5,
			},
		}); err == nil || !strings.Contains(err.Error(), "time conflict") {
			t.Fatalf("Expected error containing \"time conflict\", got: %+v", err)
		}
	})

	// Ensure that the push occurs automatically after a timeout
	t.Run("Push after timeout", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
			Labels: map[string]string{
				"foo": "bar",
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		mi.DoAndWait(t, 2, func() {
			mockClock.SetNow(time.Unix(10, 0))
		})

		reports := mi.Reports()
		if len(reports) != 2 {
			t.Fatalf("Expected push of 2 reports after timeout, got: %+v", reports)
		}

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(4, 0),
			EndTime:   time.Unix(5, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(6, 0),
			EndTime:   time.Unix(7, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
			Labels: map[string]string{
				"foo": "bar",
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		mi.DoAndWait(t, 4, func() {
			mockClock.SetNow(time.Unix(30, 0))
		})

		reports = mi.Reports()
		if len(reports) != 2 {
			t.Fatalf("Expected push of 2 reports after timeout, got: %+v", reports)
		}
	})

	// Ensure that a push happens when the aggregator is Released
	t.Run("Push after Release", func(t *testing.T) {
		mockClock := testlib.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		mi := testlib.NewMockInput()
		a := newAggregator(metric, bufTime, mi, persistence.NewMemoryPersistence(), mockClock)

		if err := a.AddReport(metrics.MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		a.Release()

		if len(mi.Reports()) == 0 {
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
