package endpoint

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
	"ubbagent/clock"
	"ubbagent/metrics"
	"ubbagent/persistence"
)

const (
	testMinDelay = 2 * time.Second
	testMaxDelay = 60 * time.Second
)

type mockReport struct {
	batch metrics.MetricBatch
}

type mockEndpoint struct {
	name      string
	sendErr   error
	buildErr  error
	sent      chan EndpointReport
	sendCalls int
	sendMutex sync.Mutex
	waitChan  chan bool
}

func (ep *mockEndpoint) Name() string {
	return ep.name
}

func (ep *mockEndpoint) Send(report EndpointReport) error {
	ep.sendMutex.Lock()
	ep.sendCalls++
	if ep.sendErr == nil {
		ep.sent <- report
	}
	if ep.waitChan != nil {
		ep.waitChan <- true
		ep.waitChan = nil
	}
	ep.sendMutex.Unlock()
	return ep.sendErr
}

func (ep *mockEndpoint) BuildReport(mb metrics.MetricBatch) (EndpointReport, error) {
	if ep.buildErr != nil {
		return nil, ep.buildErr
	}
	return mockReport{batch: mb}, nil
}

func (ep *mockEndpoint) EmptyReport() EndpointReport {
	return mockReport{}
}

func (ep *mockEndpoint) doAndWait(t *testing.T, f func()) {
	waitChan := make(chan bool, 1)
	ep.sendMutex.Lock()
	ep.waitChan = waitChan
	f()
	ep.sendMutex.Unlock()
	select {
	case <-waitChan:
	case <-time.After(5 * time.Second):
		t.Fatal("doAndWait: nothing happened after 5 seconds")
	}
}

func newMockEndpoint(name string) *mockEndpoint {
	return &mockEndpoint{
		name: name,
		sent: make(chan EndpointReport, 100),
	}
}

func TestRetryingSender(t *testing.T) {
	batch1 := metrics.MetricBatch{
		{
			Name:      "int-metric",
			Value:     metrics.MetricValue{IntValue: 10},
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
		},
		{
			Name:      "int-metric",
			Value:     metrics.MetricValue{IntValue: 20},
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
		},
	}
	batch2 := []metrics.MetricReport{
		{
			Name:      "int-metric",
			Value:     metrics.MetricValue{IntValue: 30},
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
		},
	}

	t.Run("report build failure", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		rs := newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		endpoint.buildErr = errors.New("build failure")
		_, err := rs.Prepare(batch1)
		if err == nil || err.Error() != endpoint.buildErr.Error() {
			t.Fatalf("build error: expected: %v, got: %v", endpoint.buildErr, err)
		}
	})

	t.Run("empty queue sends immediately", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		rs := newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		mc.SetNow(time.Unix(2000, 0))
		ps, err := rs.Prepare(batch1)
		if err != nil {
			t.Fatalf("empty queue: unexpected error preparing report: %+v", err)
		}
		if err := ps.Send(); err != nil {
			t.Fatalf("empty queue: unexpected error sending report: %+v", err)
		}
		select {
		case rep := <-endpoint.sent:
			mr := rep.(mockReport)
			if !reflect.DeepEqual(mr.batch, batch1) {
				t.Fatalf("Sent report contains incorrect batch: expected: %+v got: %+v", batch1, mr.batch)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Failed to receive sent report after 5 seconds")
		}
	})

	t.Run("failed send is retried with exponential delay", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		endpoint.sendErr = errors.New("Send failure")
		rs := newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		now := time.Unix(3000, 0)
		mc.SetNow(now)
		ps, err := rs.Prepare(batch1)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		if err := ps.Send(); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		// Exponential delay minimum is 2 seconds (defined above as testMinDelay)
		var expectedDelays = []int{2, 4, 8, 16, 32}
		for _, delay := range expectedDelays {
			expectedNext := now.Add(time.Duration(delay) * time.Second)
			waitForNewTimer(mc, expectedNext, t)
			now = expectedNext
			mc.SetNow(now)
		}
		if endpoint.sendCalls != 5 {
			t.Fatalf("Expected 5 send calls, got: %v", endpoint.sendCalls)
		}
	})

	t.Run("queue is cleared after success", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		rs := newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		endpoint.sendErr = errors.New("Send failure")
		mc.SetNow(time.Unix(4000, 0))

		ps1, err := rs.Prepare(batch1)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		ps2, err := rs.Prepare(batch2)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		if err := ps1.Send(); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		if err := ps2.Send(); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}

		endpoint.doAndWait(t, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		if len(endpoint.sent) != 0 {
			t.Fatalf("Send chan size should be 0, but was: %v", len(endpoint.sent))
		}

		endpoint.sendErr = nil
		endpoint.doAndWait(t, func() {
			mc.SetNow(time.Unix(4500, 0))
		})

		// The sender should have cleared its queue. Our sent chan should be length 2.
		if len(endpoint.sent) != 2 {
			t.Fatalf("Send chan size should be 2, but was: %v", len(endpoint.sent))
		}
	})

	t.Run("endpoint loads state after restart", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		rs := newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		endpoint.sendErr = errors.New("Send failure")
		mc.SetNow(time.Unix(5000, 0))

		endpoint.doAndWait(t, func() {
			ps, err := rs.Prepare(batch1)
			if err != nil {
				t.Fatalf("Unexpected prepare error: %+v", err)
			}
			if err := ps.Send(); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})
		rs.Close()

		// Create a new endpoint and sender, but keep the previous persistence. The sender should
		// load state and send the reports, and the new endpoint should not respond with errors.
		endpoint = newMockEndpoint("mockep")
		endpoint.doAndWait(t, func() {
			mc.SetNow(time.Unix(5500, 0))
			rs = newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		})

		// The sender should have cleared its queue. Our sent chan should be length 2.
		if len(endpoint.sent) == 0 {
			t.Fatal("Send chan should not be empty")
		}
	})
}

// waitForNewTimer waits for up to ~5 seconds for a timer to be set on mc with time t.
func waitForNewTimer(mc clock.MockClock, expected time.Time, t *testing.T) {
	for i := 0; i < 5000; i++ {
		if mc.GetNextFireTime() == expected {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatalf("No timer set for expected time %v after delay", expected)
}
