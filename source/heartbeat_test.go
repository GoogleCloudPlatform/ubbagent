// Copyright 2018 Google LLC
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

package source

import (
	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TODO(volkman): extract reusable mock objects into a separate test package.
type mockSender struct {
	Reports  []metrics.MetricReport // must hold sendMutex to read/write
	Used     bool
	Released bool

	sendMutex sync.Mutex
	waitChan  chan bool
	waitDone  chan bool
}

func (s *mockSender) Send(report metrics.StampedMetricReport) error {
	s.sendMutex.Lock()
	s.Reports = append(s.Reports, report.MetricReport)
	if s.waitChan != nil {
		s.waitChan <- true
		if <-s.waitDone {
			s.waitChan = nil
			s.waitDone = nil
		}
	}
	s.sendMutex.Unlock()
	return nil
}

func (s *mockSender) Endpoints() (empty []string) {
	return
}

func (s *mockSender) Use() {
	s.Used = true
}

func (s *mockSender) Release() error {
	s.Released = true
	return nil
}

func (s *mockSender) getReports() (reports []metrics.MetricReport) {
	s.sendMutex.Lock()
	reports = s.Reports
	s.Reports = []metrics.MetricReport{}
	s.sendMutex.Unlock()
	return
}

func (s *mockSender) doAndWait(t *testing.T, expected int, f func()) {
	waitChan := make(chan bool, 1)
	waitDone := make(chan bool, 1)
	s.sendMutex.Lock()
	s.waitChan = waitChan
	s.waitDone = waitDone
	f()
	s.sendMutex.Unlock()
	count := 0
	end := time.Now().Add(5 * time.Second)
	for {
		select {
		case <-waitChan:
			count += 1
			if count >= expected {
				s.waitDone <- true
				return
			}
			s.waitDone <- false
		case <-time.After(end.Sub(time.Now())):
			t.Fatal("doAndWait: nothing happened after 5 seconds")
		}
	}
}

func newMockSender() *mockSender {
	ms := &mockSender{}
	ms.getReports()
	return ms
}

func TestHeartbeat(t *testing.T) {

	def := metrics.Definition{
		Name: "instanceSeconds",
		Type: metrics.IntType,
	}

	heartbeat := config.Heartbeat{
		IntervalSeconds: 10,
		Value: metrics.MetricValue{
			IntValue: 10,
		},
		Labels: map[string]string{
			"foo": "bar",
		},
	}

	t.Run("sender used and released", func(t *testing.T) {
		mc := clock.NewMockClock()
		s := newMockSender()
		hb := newHeartbeat(def, heartbeat, s, mc)

		if s.Used != true {
			t.Fatalf("expected s.Used == true")
		}
		if s.Released != false {
			t.Fatalf("expected s.Released == false")
		}

		hb.Release()

		if s.Released != true {
			t.Fatalf("expected s.Released == true")
		}
	})

	t.Run("proper value and labels sent", func(t *testing.T) {
		mc := clock.NewMockClock()
		s := newMockSender()
		hb := newHeartbeat(def, heartbeat, s, mc)

		s.doAndWait(t, 1, func() {
			mc.SetNow(mc.Now().Add(10 * time.Second))
		})

		reports := s.getReports()
		if len(reports) != 1 {
			t.Fatalf("expected 1 report")
		}

		if !reflect.DeepEqual(reports[0].Value, heartbeat.Value) {
			t.Fatalf("unexpected report value")
		}

		if !reflect.DeepEqual(reports[0].Labels, heartbeat.Labels) {
			t.Fatalf("unexpected report labels")
		}

		hb.Release()
	})

	t.Run("no coverage gap", func(t *testing.T) {
		mc := clock.NewMockClock()
		s := newMockSender()
		hb := newHeartbeat(def, heartbeat, s, mc)

		// First fire
		s.doAndWait(t, 1, func() {
			mc.SetNow(mc.Now().Add(10 * time.Second))
		})
		// Second fire; timer is a bit late
		s.doAndWait(t, 1, func() {
			mc.SetNow(mc.Now().Add(11 * time.Second))
		})
		// Third fire; should still be on schedule (10 + 11 + 9 == 30)
		s.doAndWait(t, 1, func() {
			mc.SetNow(mc.Now().Add(9 * time.Second))
		})
		hb.Release()

		reports := s.getReports()
		if len(reports) != 3 {
			t.Fatalf("expected 3 reports")
		}

		if reports[1].StartTime != reports[0].EndTime || reports[2].StartTime != reports[1].EndTime {
			t.Fatalf("coverage gap")
		}

		expected := 10 * time.Second
		for _, v := range reports {
			got := v.EndTime.Sub(v.StartTime)
			if got != expected {
				t.Fatalf("expected interval of %v, got %v", expected, got)
			}
		}
	})
}
