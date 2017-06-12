package metrics

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"ubbagent/clock"
	"ubbagent/config"
	"ubbagent/persistence"
)

type mockPreparedSend struct {
	ms *mockSender
	mb MetricBatch
}

func (ps *mockPreparedSend) Send() error {
	return ps.ms.send(ps.mb)
}

type mockSender struct {
	reports   MetricBatch
	sendMutex sync.Mutex
	sendErr   error
	waitChan  chan bool
}

func (s *mockSender) Prepare(mb MetricBatch) (PreparedSend, error) {
	return &mockPreparedSend{ms: s, mb: mb}, nil
}

func (s *mockSender) send(mb MetricBatch) error {
	s.sendMutex.Lock()
	s.reports = mb
	if s.waitChan != nil {
		s.waitChan <- true
		s.waitChan = nil
	}
	s.sendMutex.Unlock()
	return s.sendErr
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

func newMockSender() *mockSender {
	return &mockSender{}
}

func TestNewAggregator(t *testing.T) {
	t.Run("Load previous state", func(t *testing.T) {
		// Ensures that a new aggregator loads previous state
		p := persistence.NewMemoryPersistence()

		conf := &config.Metrics{
			BufferSeconds: 10,
			Definitions: []config.MetricDefinition{
				{
					Name: "int-metric1",
					Type: "int",
				},
				{
					Name: "int-metric2",
					Type: "int",
				},
				{
					Name: "int-metric3",
					Type: "int",
				},
			},
		}

		report1 := MetricReport{
			Name:      "int-metric1",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: MetricValue{
				IntValue: 10,
			},
		}

		report2 := MetricReport{
			Name:      "int-metric2",
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
			Value: MetricValue{
				IntValue: 333,
			},
		}

		report3 := MetricReport{
			Name:      "int-metric3",
			StartTime: time.Unix(100, 0),
			EndTime:   time.Unix(110, 0),
			Value: MetricValue{
				IntValue: 555,
			},
		}

		sender := newMockSender()
		mockClock := clock.NewMockClock()
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(conf, sender, p, mockClock)

		if err := a.AddReport(report1); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}
		if err := a.AddReport(report2); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		if len(sender.reports) > 0 {
			t.Fatalf("Expected no reports, got: %+v", sender.reports)
		}

		// We set a send error on the mock sender to prevent the aggregator from successfully sending
		// its state at close. A new aggregator created with the same persistence should start with
		// the previous state.
		sender.sendErr = errors.New("send failure")
		a.Close()

		// Construct a new aggregator using the same persistence.
		a = newAggregator(conf, sender, p, mockClock)

		sender.doAndWait(t, func() {
			sender.sendErr = nil
			mockClock.SetNow(time.Unix(100, 0))
		})

		expected := []MetricReport{report1, report2}
		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}

		sender.sendErr = errors.New("send failure")
		a.Close()

		// Create one more aggregator and ensure it doesn't start with previous state.
		a = newAggregator(conf, sender, p, mockClock)

		if err := a.AddReport(report3); err != nil {
			t.Fatalf("Unexpected error when adding report: %+v", err)
		}

		sender.doAndWait(t, func() {
			sender.sendErr = nil
			mockClock.SetNow(time.Unix(200, 0))
		})

		expected = []MetricReport{report3}
		if !equalUnordered(sender.reports, expected) {
			t.Fatalf("Aggregated reports: expected: %+v, got: %+v", expected, sender.reports)
		}
		a.Close()
	})
}

func TestAggregator_AddReport(t *testing.T) {
	conf := &config.Metrics{
		BufferSeconds: 1,
		Definitions: []config.MetricDefinition{
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

	sender := newMockSender()
	mockClock := clock.NewMockClock()

	// Add a report to a zero-state aggregator
	t.Run("Zero state", func(t *testing.T) {
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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
		sender.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

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
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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
		sender.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

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
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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
		sender.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

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
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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
		sender.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(100, 0))
		})

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
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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
			t.Fatalf("Expected error containing \"Time conflict\", got: %+v", err)
		}
	})

	// Ensure that the push occurs automatically after a timeout
	t.Run("Push after timeout", func(t *testing.T) {
		sender.reports = []MetricReport{}
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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

		sender.doAndWait(t, func() {
			mockClock.SetNow(time.Unix(10, 0))
		})

		if len(sender.reports) == 0 {
			t.Fatal("Expected push after timeout, but sender contains no reports")
		}
	})

	// Ensure that a push happens when the aggregator is closed
	t.Run("Push after close", func(t *testing.T) {
		sender.reports = []MetricReport{}
		sender.sendErr = nil
		mockClock.SetNow(time.Unix(0, 0))
		a := newAggregator(conf, sender, persistence.NewMemoryPersistence(), mockClock)

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

		a.Close()

		if len(sender.reports) == 0 {
			t.Fatal("Expected push after close, but sender contains no reports")
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
