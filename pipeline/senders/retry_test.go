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

package senders

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

const (
	testMinDelay = 2 * time.Second
	testMaxDelay = 60 * time.Second
)

func TestRetryingSender(t *testing.T) {
	report1 := metrics.StampedMetricReport{
		Id: "report1",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric",
			Value:     metrics.MetricValue{Int64Value: testlib.Int64Ptr(10)},
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
		},
	}
	report2 := metrics.StampedMetricReport{
		Id: "report2",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric",
			Value:     metrics.MetricValue{Int64Value: testlib.Int64Ptr(30)},
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
		},
	}
	report3 := metrics.StampedMetricReport{
		Id: "report3",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric",
			Value:     metrics.MetricValue{Int64Value: testlib.Int64Ptr(30)},
			StartTime: time.Unix(20, 0),
			EndTime:   time.Unix(21, 0),
		},
	}

	t.Run("report build failure", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		buildErr := errors.New("build failure")
		ep.SetBuildErr(buildErr)
		err := rs.Send(report1)
		if err == nil || err.Error() != buildErr.Error() {
			t.Fatalf("build error: expected: %v, got: %v", buildErr, err)
		}
	})

	t.Run("empty queue sends immediately", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		mc.SetNow(time.Unix(2000, 0))
		ep.DoAndWait(t, 1, func() {
			if err := rs.Send(report1); err != nil {
				t.Fatalf("empty queue: unexpected error sending report: %+v", err)
			}
		})
		rep := ep.Reports()[0]
		if !reflect.DeepEqual(rep.StampedMetricReport, report1) {
			t.Fatalf("Sent report contains incorrect report: expected: %+v got: %+v", report1, rep.StampedMetricReport)
		}
	})

	t.Run("failed send is retried with exponential backoff", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		ep.SetSendErr(errors.New("send failure"))
		rs := newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		now := time.Unix(3000, 0)
		mc.SetNow(now)
		if err := rs.Send(report1); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		// Exponential delay minimum is 2 seconds (defined above as testMinDelay)
		var expectedDelays = []time.Duration{2, 4, 8, 16, 32}
		for _, delay := range expectedDelays {
			expectedNext := now.Add(delay * time.Second)
			now = waitForNewTimer(mc, expectedNext, expectedNext.Add(1*time.Second), t)
			mc.SetNow(now)
		}

		// Wait for the last one.
		expectedNext := now.Add(testMaxDelay)
		waitForNewTimer(mc, expectedNext, expectedNext.Add(1*time.Second), t)

		if want, got := int32(6), ep.Calls(); want != got {
			t.Fatalf("Expected %v send calls, got: %v", want, got)
		}
	})

	t.Run("queue is cleared after success", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.SetSendErr(errors.New("send failure"))
		mc.SetNow(time.Unix(4000, 0))

		if err := rs.Send(report1); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		if err := rs.Send(report2); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}

		ep.DoAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		r := ep.Reports()
		if len(r) != 0 {
			t.Fatalf("Report count should be 0, but was: %v", len(r))
		}

		ep.DoAndWait(t, 4, func() {
			ep.SetSendErr(nil)
			mc.SetNow(time.Unix(4500, 0))
		})

		// The sender should have cleared its queue. Our sent chan should be length 2.
		r = ep.Reports()
		if len(r) != 2 {
			t.Fatalf("Report count should be 2, but was: %v", len(r))
		}
	})

	t.Run("non-transient error results in drop of request from queue", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.SetSendErr(errors.New("non-fatal"))
		mc.SetNow(time.Unix(4000, 0))

		ep.DoAndWait(t, 1, func() {
			if err := rs.Send(report1); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
			if err := rs.Send(report2); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		r := ep.Reports()
		if len(r) != 0 {
			t.Fatalf("Report count should be 0, but was: %v", len(r))
		}

		// Set a fatal error and advance the clock. Two sends should fail completely, bringing the total
		// number of sends to 3.
		ep.DoAndWait(t, 3, func() {
			ep.SetSendErr(errors.New("FATAL"))
			mc.SetNow(time.Unix(4500, 0))
		})

		// Check the sent chan size - it should still be empty since a send error is set.
		r = ep.Reports()
		if len(r) != 0 {
			t.Fatalf("Report count should be 0, but was: %v", len(r))
		}

		// Now we clear the error and make sure a successful send makes it to our sent chan.
		ep.DoAndWait(t, 4, func() {
			ep.SetSendErr(nil)
			if err := rs.Send(report3); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})

		// Our sent chan should be length 1.
		r = ep.Reports()
		if len(r) != 1 {
			t.Fatalf("Report count should be 1, but was: %v", len(r))
		}
	})

	t.Run("Failing entry expires", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		sr := testlib.NewMockStatsRecorder()
		rs := newRetryingSender(ep, persist, sr, mc, testMinDelay, testMaxDelay)
		ep.SetSendErr(errors.New("send failure"))
		mc.SetNow(time.Unix(4000, 0))

		if err := rs.Send(report1); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		if err := rs.Send(report2); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}

		ep.DoAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		// Check the sent chan size - it should still be empty since the mock endpoint always errors on
		// sends.
		if want, got := 0, len(ep.Reports()); want != got {
			t.Fatalf("len(ep.Reports()): want=%+v, got=%+v", want, got)
		}
		if want, got := 0, len(sr.Succeeded()); want != got {
			t.Fatalf("len(sr.succeeded): want=%+v, got=%+v", want, got)
		}
		if want, got := 0, len(sr.Failed()); want != got {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}

		// Set the time far in the future. Both entries should retry one more time and then expire.
		sr.DoAndWait(t, 2, func() {
			mc.SetNow(time.Unix(100000, 0))
		})

		// Still 0 sends since both entries expired.
		if want, got := 0, len(ep.Reports()); want != got {
			t.Fatalf("len(ep.Reports): want=%+v, got=%+v", want, got)
		}
		if want, got := 0, len(sr.Succeeded()); want != got {
			t.Fatalf("len(sr.succeeded): want=%+v, got=%+v", want, got)
		}
		if want, got := 2, len(sr.Failed()); want != got {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}
	})

	t.Run("endpoint loads state after restart", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		rs := newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		ep.SetSendErr(errors.New("send failure"))
		mc.SetNow(time.Unix(5000, 0))

		ep.DoAndWait(t, 1, func() {
			if err := rs.Send(report1); err != nil {
				t.Fatalf("Unexpected send error: %+v", err)
			}
		})
		rs.Release()

		// Create a new endpoint and sender, but keep the previous persistence. The sender should
		// load state and send the reports, and the new endpoint should not respond with errors.
		ep = testlib.NewMockEndpoint("mockep")
		ep.DoAndWait(t, 1, func() {
			mc.SetNow(time.Unix(5500, 0))
			rs = newRetryingSender(ep, persist, testlib.NewMockStatsRecorder(), mc, testMinDelay, testMaxDelay)
		})

		// The sender should have cleared its queue. Our sent chan should be length 2.
		if len(ep.Reports()) == 0 {
			t.Fatal("Send chan should not be empty")
		}
	})

	t.Run("send stats are registered", func(t *testing.T) {
		persist := persistence.NewMemoryPersistence()
		mc := testlib.NewMockClock()
		ep := testlib.NewMockEndpoint("mockep")
		sr := testlib.NewMockStatsRecorder()
		rs := newRetryingSender(ep, persist, sr, mc, testMinDelay, testMaxDelay)
		mc.SetNow(time.Unix(4000, 0))

		if err := rs.Send(report1); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		if err := rs.Send(report2); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}

		sr.DoAndWait(t, 2, func() {
			mc.SetNow(time.Unix(4300, 0))
		})

		if want, got := []testlib.RecordedEntry{{Id: report1.Id, Handler: "mockep"}, {Id: report2.Id, Handler: "mockep"}}, sr.Succeeded(); !reflect.DeepEqual(want, got) {
			t.Fatalf("sr.succeeded: want=%+v, got=%+v", want, got)
		}

		if want, got := 0, len(sr.Failed()); want != got {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}

		// Now we set a send failure and try again. The failure should be registered.

		ep.SetSendErr(errors.New("FATAL"))
		if err := rs.Send(report3); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}

		sr.DoAndWait(t, 3, func() {
			mc.SetNow(time.Unix(4800, 0))
		})

		// No changes to sr.succeeded
		if want, got := []testlib.RecordedEntry{{Id: report1.Id, Handler: "mockep"}, {Id: report2.Id, Handler: "mockep"}}, sr.Succeeded(); !reflect.DeepEqual(want, got) {
			t.Fatalf("sr.succeeded: want=%+v, got=%+v", want, got)
		}

		// There should now be one failure.
		if want, got := []testlib.RecordedEntry{{Id: report3.Id, Handler: "mockep"}}, sr.Failed(); !reflect.DeepEqual(want, got) {
			t.Fatalf("len(sr.failed): want=%+v, got=%+v", want, got)
		}
	})

	t.Run("multiple usages", func(t *testing.T) {
		ep := testlib.NewMockEndpoint("mockep")
		sr := testlib.NewMockStatsRecorder()
		rs := newRetryingSender(ep, persistence.NewMemoryPersistence(), sr, testlib.NewMockClock(), testMinDelay, testMaxDelay)

		// Test multiple usages of the RetryingSender.
		rs.Use()
		rs.Use()

		rs.Release() // Usage count should still be 1.
		if ep.Released {
			t.Fatal("endpoint.released expected to be false")
		}

		rs.Release() // Usage count should be 0; endpoint should be released.
		if !ep.Released {
			t.Fatal("endpoint.released expected to be true")
		}
	})

}

// waitForNewTimer waits for up to ~5 seconds for a timer to be set on mc with a time between [lower,upper).
func waitForNewTimer(mc testlib.MockClock, lower, upper time.Time, t *testing.T) (result time.Time) {
	for i := 0; i < 5000; i++ {
		next := mc.GetNextFireTime()
		if !next.Before(lower) && next.Before(upper) {
			result = next
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatalf("No timer set for expected time range [%v,%v) after delay", lower, upper)
	return
}
