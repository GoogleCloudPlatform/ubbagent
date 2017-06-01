package metrics

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"ubbagent/clock"
	"ubbagent/persistence"
)

type mockSender struct {
	reports MetricBatch
}

func (s *mockSender) Send(mb MetricBatch) error {
	s.reports = mb
	return nil
}

func TestNewAggregator(t *testing.T) {
	t.Run("Load previous state", func(t *testing.T) {
		// Ensures that a new aggregator loads previous state
		p := persistence.NewMemoryPersistence()

		conf := Config{
			BufferSeconds: 10,
			MetricDefinitions: []MetricDefinition{
				{
					Name: "int-metric",
					Type: "int",
				},
			},
		}

		report := MetricReport{
			Name:      "int-metric",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}

		sender := &mockSender{}
		mockClock := clock.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		a := NewAggregator(conf, sender, p)
		a.clock = mockClock

		if err := a.AddReport(report); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		if len(sender.reports) > 0 {
			t.Fatalf("Expected no reports, got: %+v", sender.reports)
		}

		// Construct a new aggregator using the same persistence.
		a = NewAggregator(conf, sender, p)
		a.clock = mockClock

		mockClock.SetNow(time.Unix(100, 0))
		a.Push()

		expected := []MetricReport{report}
		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}

		// One more new aggregator shouldn't have anymore aggregated reports after the previous push.
		sender = &mockSender{}
		a = NewAggregator(conf, sender, p)
		a.clock = mockClock
		mockClock.SetNow(time.Unix(200, 0))
		a.Push()

		if len(sender.reports) > 0 {
			t.Fatalf("Expected no more reports, got: %+v", sender.reports)
		}
	})
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
		a := NewAggregator(conf, sender, persistence.NewMemoryPersistence())
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
