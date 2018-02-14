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

package inputs

import (
	"fmt"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
)

// Type selector is a pipeline.Input that routes a MetricReport to another pipeline.Input based on
// the metric name.
type selector struct {
	// Map of metric names to pipeline.Input objects.
	inputs  map[string]pipeline.Input
	tracker pipeline.UsageTracker
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
		components := make([]pipeline.Component, len(s.inputs))
		i := 0
		for _, v := range s.inputs {
			components[i] = v
			i++
		}
		return pipeline.ReleaseAll(components)
	})
}

// NewSelector creates an Input that selects from the given inputs based on metric name. The inputs
// parameter is a map of metric name to the corresponding Input that handles it.
func NewSelector(inputs map[string]pipeline.Input) pipeline.Input {
	for _, a := range inputs {
		a.Use()
	}
	return &selector{inputs: inputs}
}

type callbackInput struct {
	delegate pipeline.Input
	shutdown func() error
	tracker  pipeline.UsageTracker
}

func (p *callbackInput) AddReport(report metrics.MetricReport) error {
	return p.delegate.AddReport(report)
}

func (p *callbackInput) Use() {
	p.tracker.Use()
}

func (p *callbackInput) Release() error {
	return p.tracker.Release(func() error {
		callbackErr := p.shutdown()
		releaseError := p.delegate.Release()
		return multierror.Append(callbackErr, releaseError).ErrorOrNil()
	})
}

// NewCallbackInput creates an Input that calls the given shutdown hook when the Input is released.
// Shutdown is called before the Input's own delegate is released.
func NewCallbackInput(delegate pipeline.Input, shutdown func() error) pipeline.Input {
	delegate.Use()
	return &callbackInput{delegate: delegate, shutdown: shutdown}
}

type labelingInput struct {
	pipeline.Component
	delegate pipeline.Input
	labels   map[string]string
}

func (i *labelingInput) AddReport(report metrics.MetricReport) error {
	for k, v := range i.labels {
		if _, exists := report.Labels[k]; exists {
			glog.Warningf("labelingInput: received report that already had label '%v'; skipping", k)
			continue
		}
		if report.Labels == nil {
			report.Labels = make(map[string]string)
		}
		report.Labels[k] = v
	}
	return i.delegate.AddReport(report)
}

// NewLabelingInput creates an Input that adds the given additional labels to incoming
// MetricReports before passing reports to the given delegate. If a report already contains a label
// with the same name, the original label is retained and a warning is logged.
func NewLabelingInput(delegate pipeline.Input, labels map[string]string) pipeline.Input {
	return &labelingInput{Component: delegate, delegate: delegate, labels: labels}
}
