package endpoint_test

import (
	"errors"
	"testing"
	"time"
	"ubbagent/endpoint"
	"ubbagent/metrics"
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

func (s *mockSender) Prepare(mb metrics.MetricBatch) (metrics.PreparedSend, error) {
	s.prepareCalled = true
	if s.prepareErr != nil {
		return nil, s.prepareErr
	}
	return &mockPreparedSend{ms: s}, nil
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
		ds := endpoint.NewDispatcher([]metrics.Sender{ms1, ms2})
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
		ds := endpoint.NewDispatcher([]metrics.Sender{ms1, ms2})
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
		ms2.sendErr = errors.New("test")
		ds := endpoint.NewDispatcher([]metrics.Sender{ms1, ms2})
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
		if err.Error() != "test" {
			t.Fatalf("Expected error message to be 'test', got: %v", err.Error())
		}
	})
}
