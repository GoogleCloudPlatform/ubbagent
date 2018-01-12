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
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"sync"
)

// Metrics contains the metric definitions that the agent expects to receive.
type Metrics struct {
	// The number of seconds that metrics should be aggregated prior to forwarding
	BufferSeconds int64

	// The list of reportable metrics
	Definitions []metrics.Definition `json:"definitions"`

	// Private cache of definitions by name for faster lookup.
	initOnce          sync.Once
	definitionsByName map[string]*metrics.Definition
}

// GetMetricDefinition returns the metrics.Definition with the given name, or nil if it does not
// exist.
func (c *Metrics) GetMetricDefinition(name string) *metrics.Definition {
	c.initOnce.Do(func() {
		c.definitionsByName = make(map[string]*metrics.Definition)
		for i := range c.Definitions {
			def := &c.Definitions[i]
			c.definitionsByName[def.Name] = def
		}
	})
	return c.definitionsByName[name]
}

// Validate checks validity of metric configuration. Specifically, it must not contain duplicate
// metric definitions, and metric definitions must specify valid type names.
func (m *Metrics) Validate(c *Config) error {
	usedNames := make(map[string]bool)
	for _, def := range m.Definitions {
		if err := def.Validate(); err != nil {
			return err
		}
		if usedNames[def.Name] {
			return errors.New(fmt.Sprintf("metric %v: duplicate name: %v", def.Name, def.Name))
		}
		usedNames[def.Name] = true
	}
	return nil
}
