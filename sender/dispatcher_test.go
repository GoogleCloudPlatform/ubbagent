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

package sender_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/sender"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
)

type mockPreparedSend struct {
	ms *mockSender
}

func (ps *mockPreparedSend) Send() error {
	ps.ms.sendCalled = true
	return ps.ms.sendErr
}

type mockSender struct {
	id            string
	prepareErr    error
	sendErr       error
	prepareCalled bool
	sendCalled    bool
	released      bool
}

func (s *mockSender) Prepare(reports ...metrics.StampedMetricReport) (sender.PreparedSend, error) {
	s.prepareCalled = true
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	return &mockPreparedSend{ms: s}, nil
}

func (s *mockSender) Endpoints() []string {
	return []string{s.id}
}

func (s *mockSender) Use() {}

func (s *mockSender) Release() error {
	s.released = true
	return nil
}

type mockStatsRecorder struct {
	registered map[string][]string
}

func (msr *mockStatsRecorder) Register(id string, handlers ...string) {
	if msr.registered == nil {
		msr.registered = make(map[string][]string)
	}
	msr.registered[id] = handlers
}

func (msr *mockStatsRecorder) SendSucceeded(id string, handler string) {}
func (msr *mockStatsRecorder) SendFailed(id string, handler string)    {}

func TestDispatcher(t *testing.T) {
	report := metrics.StampedMetricReport{
		Id: "report",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric",
			Value:     metrics.MetricValue{IntValue: 30},
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
		},
	}

	t.Run("all sub-senders are invoked", func(t *testing.T) {
		ms1 := &mockSender{id: "ms1"}
		ms2 := &mockSender{id: "ms2"}
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2}, stats.NewNoopRecorder())
		s, err := ds.Prepare(report)

		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		if !ms1.prepareCalled {
			t.Fatal("ms1.prepareCalled == false")
		}
		if !ms2.prepareCalled {
			t.Fatal("ms2.prepareCalled == false")
		}

		if err := s.Send(); err != nil {
			t.Fatalf("Unexpected send error: %+v", err)
		}
		if !ms1.sendCalled {
			t.Fatal("ms1.sendCalled == false")
		}
		if !ms2.sendCalled {
			t.Fatal("ms2.sendCalled == false")
		}
	})

	t.Run("prepare failure", func(t *testing.T) {
		ms1 := &mockSender{id: "ms1"}
		ms2 := &mockSender{id: "ms2"}
		ms2.prepareErr = errors.New("test")
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2}, stats.NewNoopRecorder())
		s, err := ds.Prepare(report)
		if err == nil {
			t.Fatal("Expected prepare error, got none")
		}
		if err.Error() != "test" {
			t.Fatalf("Expected error message to be 'test', got: %v", err.Error())
		}
		if s != nil {
			t.Fatal("PreparedSend result should be nil due to prepare error")
		}
	})

	t.Run("send failure", func(t *testing.T) {
		ms1 := &mockSender{id: "ms1"}
		ms2 := &mockSender{id: "ms2"}
		ms2.sendErr = errors.New("testabcd")
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2}, stats.NewNoopRecorder())
		s, err := ds.Prepare(report)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}
		if s == nil {
			t.Fatal("PreparedSend is nil")
		}

		err = s.Send()
		if !ms1.sendCalled {
			t.Fatal("ms1.sendCalled == false")
		}
		if !ms2.sendCalled {
			t.Fatal("ms2.sendCalled == false")
		}
		if err == nil {
			t.Fatal("Expected send error, got none")
		}
		if !strings.Contains(err.Error(), "testabcd") {
			t.Fatalf("Expected error message to contain 'testabcd', got: %v", err.Error())
		}
	})

	t.Run("dispatcher returns aggregated endpoints", func(t *testing.T) {
		ms1 := &mockSender{id: "ms1"}
		ms2 := &mockSender{id: "ms2"}
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2}, stats.NewNoopRecorder())

		if want, got := []string{"ms1", "ms2"}, ds.Endpoints(); !reflect.DeepEqual(want, got) {
			t.Fatalf("ds.Endpoints(): expected %+v, got %+v", want, got)
		}
	})

	t.Run("multiple usages", func(t *testing.T) {
		s := &mockSender{id: "sender"}
		ds := sender.NewDispatcher([]sender.Sender{s}, stats.NewNoopRecorder())

		// Test multiple usages of the Dispatcher.
		ds.Use()
		ds.Use()

		ds.Release() // Usage count should still be 1.
		if s.released {
			t.Fatal("sender.released expected to be false")
		}

		ds.Release() // Usage count should be 0; sender should be released.
		if !s.released {
			t.Fatal("sender.released expected to be true")
		}
	})

	// Ensure that a StatsRecorder is notified about a send
	t.Run("Send registers with stats recorder", func(t *testing.T) {

		ms1 := &mockSender{id: "sender1"}
		ms2 := &mockSender{id: "sender2"}
		msr := &mockStatsRecorder{}
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2}, msr)

		r1 := metrics.StampedMetricReport{
			Id: "r1",
			MetricReport: metrics.MetricReport{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					IntValue: 10,
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
					DoubleValue: 10,
				},
			},
		}

		send, err := ds.Prepare(r1, r2)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %v", err)
		}
		if err := send.Send(); err != nil {
			t.Fatalf("Unexpected send error: %v", err)
		}

		expected := map[string][]string{
			"r1": []string{"sender1", "sender2"},
			"r2": []string{"sender1", "sender2"},
		}

		if want, got := expected, msr.registered; !reflect.DeepEqual(want, got) {
			t.Fatalf("Recorded stats entries: got=%+v, want=%+v", got, want)
		}
	})
}
