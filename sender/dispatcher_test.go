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
)

type mockPreparedSend struct {
	ms *mockSender
	id string
}

func (ps *mockPreparedSend) Send() error {
	ps.ms.sendCalled = true
	return ps.ms.sendErr
}

func (ps *mockPreparedSend) BatchId() string {
	return ps.id
}

func (ps *mockPreparedSend) Handlers() []string {
	return []string{ps.ms.id}
}

type mockSender struct {
	id            string
	prepareErr    error
	sendErr       error
	prepareCalled bool
	sendCalled    bool
	released      bool
}

func (s *mockSender) Prepare(mb metrics.MetricBatch) (sender.PreparedSend, error) {
	s.prepareCalled = true
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	return &mockPreparedSend{ms: s, id: mb.Id}, nil
}

func (s *mockSender) Use() {}

func (s *mockSender) Release() error {
	s.released = true
	return nil
}

func TestDispatcher(t *testing.T) {
	batch := metrics.MetricBatch{
		Id: "batch",
		Reports: []metrics.MetricReport{
			{
				Name:      "int-metric",
				Value:     metrics.MetricValue{IntValue: 30},
				StartTime: time.Unix(10, 0),
				EndTime:   time.Unix(11, 0),
			},
		},
	}

	t.Run("all sub-senders are invoked", func(t *testing.T) {
		ms1 := &mockSender{id: "ms1"}
		ms2 := &mockSender{id: "ms2"}
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2})
		s, err := ds.Prepare(batch)

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
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2})
		s, err := ds.Prepare(batch)
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
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2})
		s, err := ds.Prepare(batch)
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

	t.Run("preparedSend returns batchId and aggregated handlers", func(t *testing.T) {
		ms1 := &mockSender{id: "ms1"}
		ms2 := &mockSender{id: "ms2"}
		ds := sender.NewDispatcher([]sender.Sender{ms1, ms2})
		ps, err := ds.Prepare(batch)
		if err != nil {
			t.Fatalf("Unexpected prepare error: %+v", err)
		}

		if want, got := batch.Id, ps.BatchId(); want != got {
			t.Fatalf("ps.BatchId(): expected %v, got %v", want, got)
		}

		if want, got := []string{"ms1", "ms2"}, ps.Handlers(); !reflect.DeepEqual(want, got) {
			t.Fatalf("ps.Handlers(): expected %+v, got %+v", want, got)
		}
	})

	t.Run("multiple usages", func(t *testing.T) {
		s := &mockSender{id: "sender"}
		ds := sender.NewDispatcher([]sender.Sender{s})

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
}
