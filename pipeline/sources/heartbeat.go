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

package sources

import (
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/golang/glog"
)

type heartbeat struct {
	hb     config.Heartbeat
	input  pipeline.Input
	clock  clock.Clock
	close  chan bool
	wait   sync.WaitGroup
	sdOnce sync.Once
}

func (h *heartbeat) Shutdown() (err error) {
	h.sdOnce.Do(func() {
		h.close <- true
		h.wait.Wait()
		err = h.input.Release()
	})
	return
}

func (h *heartbeat) run(start time.Time) {
	interval := time.Duration(h.hb.IntervalSeconds) * time.Second
	end := start.Add(interval)

	running := true
	for running {
		now := h.clock.Now()
		nextFire := now.Add(end.Sub(now))
		timer := h.clock.NewTimerAt(nextFire)
		select {
		case <-timer.GetC():
			report := metrics.MetricReport{
				Name:      h.hb.Metric,
				StartTime: start,
				EndTime:   end,
				Value:     h.hb.Value,
				Labels:    h.hb.Labels,
			}
			err := h.input.AddReport(report)
			if err != nil {
				glog.Errorf("heartbeat: error sending report: %+v", err)
			}
			start = end
			end = end.Add(interval)
		case <-h.close:
			running = false
		}
		timer.Stop()
	}
	h.wait.Done()
}

func newHeartbeat(hb config.Heartbeat, input pipeline.Input, clock clock.Clock) pipeline.Source {
	input.Use()
	c := &heartbeat{hb: hb, input: input, clock: clock, close: make(chan bool, 1)}
	c.wait.Add(1)
	go c.run(clock.Now().UTC().Round(1 * time.Second))
	return c
}

func NewHeartbeat(hb config.Heartbeat, input pipeline.Input) pipeline.Source {
	return newHeartbeat(hb, input, clock.NewClock())
}
