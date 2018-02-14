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

package config

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

type Source struct {
	Name string `json:"name"`

	// oneof
	Heartbeat *Heartbeat `json:"heartbeat"`
}

func (s *Source) Validate(c *Config) error {
	if s.Name == "" {
		return errors.New("missing source name")
	}
	types := 0
	for _, v := range []Validatable{s.Heartbeat} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(c); err != nil {
			return fmt.Errorf("source %v: %v", s.Name, err)
		}
		types++
	}

	if types == 0 {
		return fmt.Errorf("source %v: missing type configuration", s.Name)
	}

	if types > 1 {
		return fmt.Errorf("source %v: multiple type configurations", s.Name)
	}

	return nil
}

type Sources []Source

func (m Sources) Validate(c *Config) error {
	usedNames := make(map[string]bool)
	for _, def := range m {
		if err := def.Validate(c); err != nil {
			return err
		}
		if usedNames[def.Name] {
			return fmt.Errorf("source %v: duplicate name: %v", def.Name, def.Name)
		}
		usedNames[def.Name] = true
	}
	return nil
}

type Heartbeat struct {
	Metric          string              `json:"metric"`
	IntervalSeconds int64               `json:"intervalSeconds"`
	Value           metrics.MetricValue `json:"value"`
	Labels          map[string]string   `json:"labels"`
}

func (h *Heartbeat) Validate(c *Config) error {
	if h.Metric == "" {
		return fmt.Errorf("metric must be specified")
	}
	d := c.Metrics.GetMetricDefinition(h.Metric)
	if d == nil {
		return fmt.Errorf("unknown metric: %v", h.Metric)
	}
	if err := h.Value.Validate(*d); err != nil {
		return err
	}
	if h.IntervalSeconds <= 0 {
		return fmt.Errorf("intervalSeconds must be > 0")
	}
	return nil
}
