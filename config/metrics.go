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

package config

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

type Metrics []Metric

type Metric struct {
	metrics.Definition `json:",inline"`
	Endpoints          []MetricEndpoint `json:"endpoints"`

	// oneof
	Reported *ReportedMetric `json:"reported"`
}

type MetricEndpoint struct {
	Name string `json:"name"`
}

type ReportedMetric struct {
	// The number of seconds that metrics should be aggregated prior to forwarding
	BufferSeconds int64 `json:"bufferSeconds"`
}

func (m *Metric) Validate(c *Config) error {
	if err := m.Definition.Validate(); err != nil {
		return err
	}
	types := 0
	for _, v := range []Validatable{m.Reported} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(c); err != nil {
			return err
		}
		types++
	}

	if types == 0 {
		return fmt.Errorf("metric %v: missing type configuration", m.Name)
	}

	if types > 1 {
		return fmt.Errorf("metric %v: multiple type configurations", m.Name)
	}

	if len(m.Endpoints) == 0 {
		return fmt.Errorf("metric %v: no endpoints defined", m.Name)
	}

	usedEndpoints := make(map[string]bool)
	for _, e := range m.Endpoints {
		if e.Name == "" {
			return fmt.Errorf("metric %v: endpoint missing name", m.Name)
		}
		if !c.Endpoints.exists(e.Name) {
			return fmt.Errorf("metric %v: endpoint does not exist: %v", m.Name, e.Name)
		}
		if usedEndpoints[e.Name] {
			return fmt.Errorf("metric %v: endpoint listed twice: %v", m.Name, e.Name)
		}
		usedEndpoints[e.Name] = true
	}

	return nil
}

func (rm *ReportedMetric) Validate(c *Config) error {
	return nil
}

// GetMetricDefinition returns the metrics.Definition with the given name, or nil if it does not
// exist.
func (m Metrics) GetMetricDefinition(name string) *metrics.Definition {
	for i := range m {
		if m[i].Name == name {
			return &m[i].Definition
		}
	}
	return nil
}

// Validate checks validity of metric configuration. Specifically, it must not contain duplicate
// metric definitions, and metric definitions must specify valid type names.
func (m Metrics) Validate(c *Config) error {
	usedNames := make(map[string]bool)
	for _, def := range m {
		if err := def.Validate(c); err != nil {
			return err
		}
		if usedNames[def.Name] {
			return errors.New(fmt.Sprintf("metric %v: duplicate name: %v", def.Name, def.Name))
		}
		usedNames[def.Name] = true
	}
	return nil
}
