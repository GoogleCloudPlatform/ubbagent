package metrics

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"ubbagent/clock"
)

type mockSender struct {
	reports []MetricReport
}

func (s *mockSender) Send(mrs []MetricReport) {
	s.reports = mrs
}

func TestAggregator_AddReport(t *testing.T) {
	conf := Config{
		BufferSeconds: 1,
		MetricDefinitions: []MetricDefinition{
			{
				Name: "int-metric",
				Type: "int",
			},
			{
				Name: "int-metric2",
				Type: "int",
			},
			{
				Name: "double-metric",
				Type: "double",
			},
		},
	}

	sender := &mockSender{}
	mockClock := clock.NewMockClock()

	// Add a report to a zero-state aggregator
	t.Run("Zero state", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mockClock.SetNow(time.Unix(100, 0))
		a.Push()

		expected := []MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: MetricValue{
					IntValue: 10,
				},
			},
		}

		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}
	})

	// Add multiple reports, testing aggregation
	t.Run("Aggregation", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: MetricValue{
				IntValue: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mockClock.SetNow(time.Unix(100, 0))
		a.Push()

		expected := []MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(3, 0),
				Value: MetricValue{
					IntValue: 15,
				},
			},
		}

		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}
	})

	// Add two reports with different names: no aggregation
	t.Run("Different names", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(MetricReport{
			Name:      "int-metric2",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: MetricValue{
				IntValue: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mockClock.SetNow(time.Unix(100, 0))
		a.Push()

		expected := []MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: MetricValue{
					IntValue: 10,
				},
			},
			{
				Name:      "int-metric2",
				StartTime: time.Unix(2, 0),
				EndTime:   time.Unix(3, 0),
				Value: MetricValue{
					IntValue: 5,
				},
			},
		}

		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}
	})

	// Add two reports with the same name but different labels: no aggregation
	t.Run("Different labels", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Labels: map[string]string{
				"key1": "value1",
			},
			Value: MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Labels: map[string]string{
				"key1": "value2",
			},
			Value: MetricValue{
				IntValue: 5,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		mockClock.SetNow(time.Unix(100, 0))
		a.Push()

		expected := []MetricReport{
			{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Labels: map[string]string{
					"key1": "value1",
				},
				Value: MetricValue{
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
				Value: MetricValue{
					IntValue: 5,
				},
			},
		}

		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}
	})

	// Add a report that fails validation: error
	t.Run("Report validation error", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(10, 0), // StartTime > EndTime -> error
			EndTime:   time.Unix(1, 0),
			Labels: map[string]string{
				"key1": "value1",
			},
			Value: MetricValue{
				IntValue: 10,
			},
		}); err == nil || !strings.Contains(err.Error(), "StartTime > EndTime") {
			t.Fatalf("Expected error containing \"StartTime > EndTime\", got: %+v", err.Error())
		}
	})

	// Add a report with a start time less than the last end time: error
	t.Run("Time conflict", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(2, 0),
			Value: MetricValue{
				IntValue: 5,
			},
		}); err == nil || !strings.Contains(err.Error(), "Time conflict") {
			t.Fatalf("Expected error containing \"Time conflict\", got: %+v", err.Error())
		}
	})

	// Ensure that the push occurs automatically after a timeout
	t.Run("Push after timeout", func(t *testing.T) {
		sender.reports = []MetricReport{}
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender)
		a.clock = mockClock
		if err := a.AddReport(MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		mockClock.SetNow(time.Unix(10, 0))
		time.Sleep(1100 * time.Millisecond)
		if len(sender.reports) == 0 {
			t.Fatal("Expected push after timeout, but sender contains no reports")
		}
	})
}

func equalUnordered(a, b []MetricReport) bool {
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
