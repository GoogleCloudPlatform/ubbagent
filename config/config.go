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
	"io/ioutil"

	"github.com/ghodss/yaml"
)

// Config contains configuration for the agent.
type Config struct {
	Identities Identities `json:"identities"`
	Metrics    Metrics    `json:"metrics"`
	Endpoints  Endpoints  `json:"endpoints"`
	Sources    Sources    `json:"sources"`
	Filters    Filters    `json:"filters"`
}

// Validation
type Validatable interface {
	Validate(*Config) error
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
	if err := c.Identities.Validate(c); err != nil {
		return err
	}
	if len(c.Metrics) == 0 {
		return errors.New("no metrics defined")
	}
	if err := c.Metrics.Validate(c); err != nil {
		return err
	}
	if len(c.Endpoints) == 0 {
		return errors.New("no endpoints defined")
	}
	if err := c.Endpoints.Validate(c); err != nil {
		return err
	}
	if err := c.Sources.Validate(c); err != nil {
		return err
	}
	if err := c.Filters.Validate(c); err != nil {
		return err
	}

	return nil
}
