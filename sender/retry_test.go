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

package sender

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"reflect"
	"sync/atomic"
)

const (
	testMinDelay = 2 * time.Second
	testMaxDelay = 60 * time.Second
)

type mockReport struct {
	Batch metrics.MetricBatch
}

type mockEndpoint struct {
	name      string
	sendErr   error
	buildErr  error
	sent      chan endpoint.EndpointReport
	sendCalls int32
	errMutex  sync.Mutex
	waitChan  chan bool
}

func (ep *mockEndpoint) Name() string {
	return ep.name
}

func (ep *mockEndpoint) Send(report endpoint.EndpointReport) error {
	atomic.AddInt32(&ep.sendCalls, 1)
	err := ep.getSendErr()
	if err == nil {
		ep.sent <- report
	}
	ep.waitChan <- true
	return err
}

func (ep *mockEndpoint) BuildReport(mb metrics.MetricBatch) (endpoint.EndpointReport, error) {
	if ep.buildErr != nil {
		return nil, ep.buildErr
	}
	return &mockReport{Batch: mb}, nil
}

func (ep *mockEndpoint) EmptyReport() endpoint.EndpointReport {
	return &mockReport{}
}

func (ep *mockEndpoint) Close() error {
	return nil
}

func (ep *mockEndpoint) setSendErr(err error) {
	ep.errMutex.Lock()
	ep.sendErr = err
	ep.errMutex.Unlock()
}

func (ep *mockEndpoint) getSendErr() (err error) {
	ep.errMutex.Lock()
	err = ep.sendErr
	ep.errMutex.Unlock()
	return
}

// doAndWait performs executes the given function and then waits until the endpoint's total number
// of Send calls reaches sends.
func (ep *mockEndpoint) doAndWait(t *testing.T, sends int32, f func()) {
	f()
	for atomic.LoadInt32(&ep.sendCalls) < sends {
		select {
		case <-ep.waitChan:
		case <-time.After(5 * time.Second):
			t.Fatal("doAndWait: nothing happened after 5 seconds")
		}
	}
}

func newMockEndpoint(name string) *mockEndpoint {
	return &mockEndpoint{
		name:     name,
		sent:     make(chan endpoint.EndpointReport, 100),
		waitChan: make(chan bool, 100),
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
			mr := rep.(*mockReport)
			if !reflect.DeepEqual(mr.Batch, batch1) {
				t.Fatalf("Sent report contains incorrect batch: expected: %+v got: %+v", batch1, mr.Batch)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Failed to receive sent report after 5 seconds")
		}
	})

	t.Run("failed send is retried with exponential delay", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		endpoint.setSendErr(errors.New("Send failure"))
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
		if atomic.LoadInt32(&endpoint.sendCalls) != 5 {
			t.Fatalf("Expected 5 send calls, got: %v", endpoint.sendCalls)
		}
	})

	t.Run("queue is cleared after success", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		endpoint := newMockEndpoint("mockep")
		rs := newRetryingSender(endpoint, persist, mc, testMinDelay, testMaxDelay)
		endpoint.setSendErr(errors.New("Send failure"))
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

		endpoint.doAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		if len(endpoint.sent) != 0 {
			t.Fatalf("Send chan size should be 0, but was: %v", len(endpoint.sent))
		}

		endpoint.doAndWait(t, 4, func() {
			endpoint.setSendErr(nil)
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
		endpoint.setSendErr(errors.New("Send failure"))
		mc.SetNow(time.Unix(5000, 0))

		endpoint.doAndWait(t, 1, func() {
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
		endpoint.doAndWait(t, 1, func() {
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
