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
	"io/ioutil"

	"github.com/ghodss/yaml"
)

// Config contains configuration for the agent.
type Config struct {
	Identity  *Identity  `json:"identity"`
	Metrics   *Metrics   `json:"metrics"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Validation
type Validatable interface {
	Validate() error
}

func Load(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Config, error) {
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) Validate() error {
	if c.Identity == nil {
		return errors.New("missing identity section")
	}
	if c.Metrics == nil {
		return errors.New("missing metrics section")
	}
	if err := c.Identity.Validate(); err != nil {
		return err
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	if len(c.Endpoints) == 0 {
		return errors.New("no endpoints defined")
	}
	usedNames := make(map[string]bool)
	for _, e := range c.Endpoints {
		if usedNames[e.Name] {
			return errors.New(fmt.Sprintf("endpoint %v: multiple endpoints with the same name", e.Name))
		}
		if err := e.Validate(); err != nil {
			return err
		}
		usedNames[e.Name] = true
	}
	return nil
}
