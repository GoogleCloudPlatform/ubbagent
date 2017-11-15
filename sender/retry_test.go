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
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
)

const (
	testMinDelay = 2 * time.Second
	testMaxDelay = 60 * time.Second
)

// Type mockReport is a mock endpoint.EndpointReport.
type mockReport struct {
	Batch metrics.MetricBatch
}

func (r mockReport) BatchId() string {
	return r.Batch.Id
}

// Type waitForCalls is a base type that provides a doAndWait function.
type waitForCalls struct {
	calls    int32
	waitChan chan bool
}

// doAndWait executes the given function and then waits until the total number of calls reaches the
// given value.
func (wfc *waitForCalls) doAndWait(t *testing.T, calls int32, f func()) {
	f()
	for atomic.LoadInt32(&wfc.calls) < calls {
		select {
		case <-wfc.waitChan:
		case <-time.After(5 * time.Second):
			t.Fatal("doAndWait: nothing happened after 5 seconds")
		}
	}
}

func (wfc *waitForCalls) called() {
	atomic.AddInt32(&wfc.calls, 1)
	wfc.waitChan <- true
}

func (wfc *waitForCalls) getCalls() int32 {
	return atomic.LoadInt32(&wfc.calls)
}

func (wfc *waitForCalls) wfcInit() {
	wfc.waitChan = make(chan bool, 100)
}

// Type mockEndpoint is a mock endpoint.Endpoint.
type mockEndpoint struct {
	waitForCalls
	name     string
	sendErr  error
	buildErr error
	sent     chan endpoint.EndpointReport
	errMutex sync.Mutex
	released bool
}

func (ep *mockEndpoint) Name() string {
	return ep.name
}

func (ep *mockEndpoint) Send(report endpoint.EndpointReport) error {
	err := ep.getSendErr()
	if err == nil {
		ep.sent <- report
	}
	ep.called()
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

func (ep *mockEndpoint) Use() {}

func (ep *mockEndpoint) Release() error {
	ep.released = true
	return nil
}

func (ep *mockEndpoint) IsTransient(err error) bool {
	return err != nil && err.Error() != "FATAL"
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

func newMockEndpoint(name string) *mockEndpoint {
	ep := &mockEndpoint{
		name: name,
		sent: make(chan endpoint.EndpointReport, 100),
	}
	ep.wfcInit()
	return ep
}

// Type mockStatsRecorder is a mock stats.StatsRecorder.
type mockStatsRecorder struct {
	waitForCalls
	mutex     sync.RWMutex
	succeeded []recordedEntry
	failed    []recordedEntry
}

type recordedEntry struct {
	batchId string
	handler string
}

func (sr *mockStatsRecorder) Register(stats.ExpectedSend) {}

func (sr *mockStatsRecorder) SendSucceeded(batchId string, handler string) {
	sr.mutex.Lock()
	sr.succeeded = append(sr.succeeded, recordedEntry{batchId, handler})
	sr.mutex.Unlock()
	sr.called()
}

func (sr *mockStatsRecorder) SendFailed(batchId string, handler string) {
	sr.mutex.Lock()
	sr.failed = append(sr.failed, recordedEntry{batchId, handler})
	sr.mutex.Unlock()
	sr.called()
}

func (sr *mockStatsRecorder) getSucceeded() []recordedEntry {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	return sr.succeeded
}

func (sr *mockStatsRecorder) getFailed() []recordedEntry {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	return sr.failed
}

func newMockStatsRecorder() *mockStatsRecorder {
	sr := &mockStatsRecorder{}
	sr.wfcInit()
	return sr
}

func TestRetryingSender(t *testing.T) {
	batch1 := metrics.MetricBatch{
		Id: "batch1",
		Reports: []metrics.MetricReport{
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
		},
	}
	batch2 := metrics.MetricBatch{
		Id: "batch2",
		Reports: []metrics.MetricReport{
			{
				Name:      "int-metric",
				Value:     metrics.MetricValue{IntValue: 30},
				StartTime: time.Unix(10, 0),
				EndTime:   time.Unix(11, 0),
			},
		},
	}
	batch3 := metrics.MetricBatch{
		Id: "batch3",
		Reports: []metrics.MetricReport{
			{
				Name:      "int-metric",
				Value:     metrics.MetricValue{IntValue: 30},
				StartTime: time.Unix(20, 0),
				EndTime:   time.Unix(21, 0),
			},
		},
	}

	t.Run("report build failure", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.buildErr = errors.New("build failure")
		_, err := rs.Prepare(batch1)
		if err == nil || err.Error() != ep.buildErr.Error() {
			t.Fatalf("build error: expected: %v, got: %v", ep.buildErr, err)
		}
	})

	t.Run("empty queue sends immediately", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		mc.SetNow(time.Unix(2000, 0))
		ps, err := rs.Prepare(batch1)
		if err != nil {
			t.Fatalf("empty queue: unexpected error preparing report: %+v", err)
		}
		if err := ps.Send(); err != nil {
			t.Fatalf("empty queue: unexpected error sending report: %+v", err)
		}
		select {
		case rep := <-ep.sent:
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
		ep := newMockEndpoint("mockep")
		ep.setSendErr(errors.New("Send failure"))
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
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
		if want, got := int32(5), ep.getCalls(); want != got {
			t.Fatalf("Expected %v send calls, got: %v", want, got)
		}
	})

	t.Run("queue is cleared after success", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.setSendErr(errors.New("Send failure"))
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

		ep.doAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		if len(ep.sent) != 0 {
			t.Fatalf("Send chan size should be 0, but was: %v", len(ep.sent))
		}

		ep.doAndWait(t, 4, func() {
			ep.setSendErr(nil)
			mc.SetNow(time.Unix(4500, 0))
		})

		// The sender should have cleared its queue. Our sent chan should be length 2.
		if len(ep.sent) != 2 {
			t.Fatalf("Send chan size should be 2, but was: %v", len(ep.sent))
		}
	})

	t.Run("non-transient error results in drop of request from queue", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.setSendErr(errors.New("non-fatal"))
		mc.SetNow(time.Unix(4000, 0))

		ps1, err := rs.Prepare(batch1)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		ps2, err := rs.Prepare(batch2)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		ps3, err := rs.Prepare(batch3)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}

		ep.doAndWait(t, 1, func() {
			if err := ps1.Send(); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
			if err := ps2.Send(); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		if len(ep.sent) != 0 {
			t.Fatalf("Send chan size should be 0, but was: %v", len(ep.sent))
		}

		// Set a fatal error and advance the clock. Two sends should fail completely, bringing the total
		// number of sends to 3.
		ep.doAndWait(t, 3, func() {
			ep.setSendErr(errors.New("FATAL"))
			mc.SetNow(time.Unix(4500, 0))
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		if len(ep.sent) != 0 {
			t.Fatalf("Send chan size should be 0, but was: %v", len(ep.sent))
		}

		// Now we clear the error and make sure a successful send makes it to our sent chan.
		ep.doAndWait(t, 4, func() {
			ep.setSendErr(nil)
			if err := ps3.Send(); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})

		// Our sent chan should be length 1.
		if len(ep.sent) != 1 {
			t.Fatalf("Send chan size should be 1, but was: %v", len(ep.sent))
		}
	})

	t.Run("Failing entry expires", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		sr := newMockStatsRecorder()
		rs := newRetryingSender(ep, persist, sr, mc, testMinDelay, testMaxDelay)
		ep.setSendErr(errors.New("Send failure"))
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

		ep.doAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		// Check the sent chan size - it should still be empty since the mock endpoint always errors on
		// sends.
		if want, got := 0, len(ep.sent); want != got {
			t.Fatalf("len(ep.sent): want=%+v, got=%+v", want, got)
		}
		if want, got := 0, len(sr.getSucceeded()); want != got {
			t.Fatalf("len(sr.succeeded): want=%+v, got=%+v", want, got)
		}
		if want, got := 0, len(sr.getFailed()); want != got {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}

		// Set the time far in the future. Both entries should retry one more time and then expire.
		sr.doAndWait(t, 2, func() {
			mc.SetNow(time.Unix(100000, 0))
		})

		// Still 0 sends since both entries expired.
		if want, got := 0, len(ep.sent); want != got {
			t.Fatalf("len(ep.sent): want=%+v, got=%+v", want, got)
		}
		if want, got := 0, len(sr.getSucceeded()); want != got {
			t.Fatalf("len(sr.succeeded): want=%+v, got=%+v", want, got)
		}
		if want, got := 2, len(sr.getFailed()); want != got {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}
	})

	t.Run("endpoint loads state after restart", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.setSendErr(errors.New("Send failure"))
		mc.SetNow(time.Unix(5000, 0))

		ep.doAndWait(t, 1, func() {
			ps, err := rs.Prepare(batch1)
			if err != nil {
				t.Fatalf("Unexpected prepare error: %+v", err)
			}
			if err := ps.Send(); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})
		rs.Release()

		// Create a new endpoint and sender, but keep the previous persistence. The sender should
		// load state and send the reports, and the new endpoint should not respond with errors.
		ep = newMockEndpoint("mockep")
		ep.doAndWait(t, 1, func() {
			mc.SetNow(time.Unix(5500, 0))
			rs = newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		})

		// The sender should have cleared its queue. Our sent chan should be length 2.
		if len(ep.sent) == 0 {
			t.Fatal("Send chan should not be empty")
		}
	})

	t.Run("preparedSend returns batchId and endpoint", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, newMockStatsRecorder(), mc, testMinDelay, testMaxDelay)

		ps1, err := rs.Prepare(batch1)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}

		if want, got := batch1.Id, ps1.BatchId(); want != got {
			t.Fatalf("ps1.BatchId(): expected %v, got %v", want, got)
		}

		if want, got := []string{"mockep"}, ps1.Handlers(); !reflect.DeepEqual(want, got) {
			t.Fatalf("ps1.Handlers(): expected %+v, got %+v", want, got)
		}
	})

	t.Run("send stats are registered", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := clock.NewMockClock()
		ep := newMockEndpoint("mockep")
		sr := newMockStatsRecorder()
		rs := newRetryingSender(ep, persist, sr, mc, testMinDelay, testMaxDelay)
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

		sr.doAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		if want, got := []recordedEntry{{batch1.Id, "mockep"}, {batch2.Id, "mockep"}}, sr.getSucceeded(); !reflect.DeepEqual(want, got) {
			t.Fatalf("sr.succeeded: want=%+v, got=%+v", want, got)
		}

		if want, got := 0, len(sr.getFailed()); want != got {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}

		// Now we set a send failure and try again. The failure should be registered.

		ep.sendErr = errors.New("FATAL")
		ps3, err := rs.Prepare(batch3)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		if err := ps3.Send(); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}

		sr.doAndWait(t, 3, func() {
			mc.SetNow(time.Unix(4800, 0))
		})

		// No changes to sr.succeeded
		if want, got := []recordedEntry{{batch1.Id, "mockep"}, {batch2.Id, "mockep"}}, sr.getSucceeded(); !reflect.DeepEqual(want, got) {
			t.Fatalf("sr.succeeded: want=%+v, got=%+v", want, got)
		}

		// There should now be one failure.
		if want, got := []recordedEntry{{batch3.Id, "mockep"}}, sr.getFailed(); !reflect.DeepEqual(want, got) {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}
	})

	t.Run("multiple usages", func(t *testing.T) {
		ep := newMockEndpoint("mockep")
		sr := newMockStatsRecorder()
		rs := newRetryingSender(ep, persistence.NewMemoryPersistence(), sr, clock.NewMockClock(), testMinDelay, testMaxDelay)

		// Test multiple usages of the RetryingSender.
		rs.Use()
		rs.Use()

		rs.Release() // Usage count should still be 1.
		if ep.released {
			t.Fatal("endpoint.released expected to be false")
		}

		rs.Release() // Usage count should be 0; endpoint should be released.
		if !ep.released {
			t.Fatal("endpoint.released expected to be true")
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
