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

package pipeline

import (
	"fmt"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

// Type selector is a pipeline.Input that routes a MetricReport to another pipeline.Input based on
// the metric name.
type selector struct {
	// Map of metric names to pipeline.Input objects.
	inputs  map[string]Input
	tracker UsageTracker
}

func (s *selector) AddReport(report metrics.MetricReport) error {
	a, ok := s.inputs[report.Name]
	if !ok {
		return fmt.Errorf("selector: unknown metric: %v", report.Name)
	}
	return a.AddReport(report)
}

// Use increments the Selector's usage count.
// See pipeline.Component.Use.
func (s *selector) Use() {
	s.tracker.Use()
}

// Release decrements the Selector's usage count. If it reaches 0, Release releases all of the
// underlying Aggregators concurrently and waits for the operations to finish.
// See pipeline.Component.Release.
func (s *selector) Release() error {
	return s.tracker.Release(func() error {
		components := make([]Component, len(s.inputs))
		i := 0
		for _, v := range s.inputs {
			components[i] = v
			i++
		}
		return ReleaseAll(components)
	})
}

// NewSelector creates an Input that selects from the given inputs based on metric name. The inputs
// parameter is a map of metric name to the corresponding Input that handles it.
func NewSelector(inputs map[string]Input) Input {
	for _, a := range inputs {
		a.Use()
	}
	return &selector{inputs: inputs}
}

type compositeInput struct {
	delegate   Input
	components []Component
	tracker    UsageTracker
}

func (p *compositeInput) AddReport(report metrics.MetricReport) error {
	return p.delegate.AddReport(report)
}

func (p *compositeInput) Use() {
	p.tracker.Use()
}

func (p *compositeInput) Release() error {
	return p.tracker.Release(func() error {
		return ReleaseAll(p.components)
	})
}

// NewCompositeInput creates a new CompositeInput. The delegate parameter is required; If a pipeline
// does not define any external inputs, delegate can be an empty Selector. The components slice is
// the collection of additional components excluding delegate.
func NewCompositeInput(delegate Input, additional []Component) *compositeInput {
	components := append(additional, delegate)
	for _, c := range components {
		c.Use()
	}
	return &compositeInput{delegate: delegate, components: components}
}
