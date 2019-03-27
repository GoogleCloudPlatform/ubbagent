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
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

func TestDispatcher(t *testing.T) {
	report := metrics.StampedMetricReport{
		Id: "report",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric",
			Value:     metrics.MetricValue{Int64Value: testlib.Int64Ptr(30)},
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
		},
	}

	t.Run("all sub-senders are invoked", func(t *testing.T) {
		ms1 := testlib.NewMockSender("ms1")
		ms2 := testlib.NewMockSender("ms2")
		ds := NewDispatcher([]pipeline.Sender{ms1, ms2}, stats.NewNoopRecorder())
		if err := ds.Send(report); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		if ms1.Calls() == 0 {
			t.Fatal("ms1.Calls() == 0")
		}
		if ms2.Calls() == 0 {
			t.Fatal("ms2.Calls() == 0")
		}
	})

	t.Run("send failure", func(t *testing.T) {
		ms1 := testlib.NewMockSender("ms1")
		ms2 := testlib.NewMockSender("ms2")
		ms2.SetSendError(errors.New("testabcd"))
		ds := NewDispatcher([]pipeline.Sender{ms1, ms2}, stats.NewNoopRecorder())
		err := ds.Send(report)
		if ms1.Calls() == 0 {
			t.Fatal("ms1.Calls() == 0")
		}
		if ms2.Calls() == 0 {
			t.Fatal("ms2.Calls() == 0")
		}
		if err == nil {
			t.Fatal("Expected send error, got none")
		}
		if !strings.Contains(err.Error(), "testabcd") {
			t.Fatalf("Expected error message to contain 'testabcd', got: %v", err.Error())
		}
	})

	t.Run("dispatcher returns aggregated endpoints", func(t *testing.T) {
		ms1 := testlib.NewMockSender("ms1")
		ms2 := testlib.NewMockSender("ms2")
		ds := NewDispatcher([]pipeline.Sender{ms1, ms2}, stats.NewNoopRecorder())

		if want, got := []string{"ms1", "ms2"}, ds.Endpoints(); !reflect.DeepEqual(want, got) {
			t.Fatalf("ds.Endpoints(): expected %+v, got %+v", want, got)
		}
	})

	t.Run("multiple usages", func(t *testing.T) {
		s := testlib.NewMockSender("sender")
		ds := NewDispatcher([]pipeline.Sender{s}, stats.NewNoopRecorder())

		// Test multiple usages of the Dispatcher.
		ds.Use()
		ds.Use()

		ds.Release() // Usage count should still be 1.
		if s.Released {
			t.Fatal("sender.released expected to be false")
		}

		ds.Release() // Usage count should be 0; sender should be released.
		if !s.Released {
			t.Fatal("sender.released expected to be true")
		}
	})

	// Ensure that a StatsRecorder is notified about a send
	t.Run("Send registers with stats recorder", func(t *testing.T) {
		ms1 := testlib.NewMockSender("sender1")
		ms2 := testlib.NewMockSender("sender2")
		msr := testlib.NewMockStatsRecorder()
		ds := NewDispatcher([]pipeline.Sender{ms1, ms2}, msr)

		r1 := metrics.StampedMetricReport{
			Id: "r1",
			MetricReport: metrics.MetricReport{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					Int64Value: testlib.Int64Ptr(10),
				},
			},
		}

		r2 := metrics.StampedMetricReport{
			Id: "r2",
			MetricReport: metrics.MetricReport{
				Name:      "double-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					DoubleValue: testlib.Float64Ptr(10),
				},
			},
		}

		if err := ds.Send(r1); err != nil {
			t.Fatalf("Unexpected send error: %v", err)
		}
		if err := ds.Send(r2); err != nil {
			t.Fatalf("Unexpected send error: %v", err)
		}

		expected := map[string][]string{
			"r1": {"sender1", "sender2"},
			"r2": {"sender1", "sender2"},
		}

		if want, got := expected, msr.Registered(); !reflect.DeepEqual(want, got) {
			t.Fatalf("Recorded stats entries: got=%+v, want=%+v", got, want)
		}
	})
}
