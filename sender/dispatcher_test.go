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
	"strings"
	"testing"
	"time"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/sender"
)

type mockPreparedSend struct {
	ms *mockSender
}

func (ps *mockPreparedSend) Send() error {
	ps.ms.sendCalled = true
	return ps.ms.sendErr
}

type mockSender struct {
	prepareErr    error
	sendErr       error
	prepareCalled bool
	sendCalled    bool
}

func (s *mockSender) Prepare(mb metrics.MetricBatch) (sender.PreparedSend, error) {
	s.prepareCalled = true
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	return &mockPreparedSend{ms: s}, nil
}

func (s *mockSender) Close() error {
	return nil
}

func TestDispatcher(t *testing.T) {
	batch := metrics.MetricBatch{
		{
			Name:      "int-metric",
			Value:     metrics.MetricValue{IntValue: 30},
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
		},
	}

	t.Run("all sub-senders are invoked", func(t *testing.T) {
		ms1 := &mockSender{}
		ms2 := &mockSender{}
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
		ms1 := &mockSender{}
		ms2 := &mockSender{}
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
		ms1 := &mockSender{}
		ms2 := &mockSender{}
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
}
