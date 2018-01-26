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
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

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
		s := testlib.NewMockSender()
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
		s := testlib.NewMockSender()
		hb := newHeartbeat(def, heartbeat, s, mc)

		s.DoAndWait(t, 1, func() {
			mc.SetNow(mc.Now().Add(10 * time.Second))
		})

		reports := s.Reports()
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
		s := testlib.NewMockSender()
		hb := newHeartbeat(def, heartbeat, s, mc)

		// First fire
		s.DoAndWait(t, 1, func() {
			mc.SetNow(mc.Now().Add(10 * time.Second))
		})
		// Second fire; timer is a bit late
		s.DoAndWait(t, 2, func() {
			mc.SetNow(mc.Now().Add(11 * time.Second))
		})
		// Third fire; should still be on schedule (10 + 11 + 9 == 30)
		s.DoAndWait(t, 3, func() {
			mc.SetNow(mc.Now().Add(9 * time.Second))
		})
		hb.Release()

		reports := s.Reports()
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
