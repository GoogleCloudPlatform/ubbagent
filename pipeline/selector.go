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

package pipeline

import (
	"sync"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/hashicorp/go-multierror"
	"fmt"
)

// Selector is a pipeline.Head that routes a MetricReport to another pipeline.Head based on the
// metric name.
type Selector struct {
	// Map of metric names to pipeline.Head objects.
	heads map[string]Head
	tracker UsageTracker
}

func (s *Selector) AddReport(report metrics.MetricReport) error {
	a, ok := s.heads[report.Name]
	if !ok {
		return fmt.Errorf("selector: unknown metric: %v", report.Name)
	}
	return a.AddReport(report)
}

// Use increments the Selector's usage count.
// See pipeline.Component.Use.
func (s *Selector) Use() {
	s.tracker.Use()
}

// Release decrements the Selector's usage count. If it reaches 0, Release releases all of the
// underlying Aggregators concurrently and waits for the operations to finish.
// See pipeline.Component.Release.
func (s *Selector) Release() error {
	return s.tracker.Release(func() error {
		errors := make([]error, len(s.heads))
		wg := sync.WaitGroup{}
		wg.Add(len(s.heads))
		var i int
		for _, a := range s.heads {
			go func(i int, a Head) {
				errors[i] = a.Release()
				wg.Done()
			}(i, a)
			i++
		}
		wg.Wait()
		return multierror.Append(nil, errors...).ErrorOrNil()
	})
}

func NewSelector(heads map[string]Head) *Selector {
	for _, a := range heads {
		a.Use()
	}
	return &Selector{heads: heads}
}
