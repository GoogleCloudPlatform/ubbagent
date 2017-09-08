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
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"

	"encoding/json"
	"github.com/ghodss/yaml"
)

const (
	IntType    = "int"
	DoubleType = "double"
)

// LiteralServiceAccountKey is a byte array type that can hold a literal json structure.
// It validates that its value is valid json upon parsing. After parsing, the contents of the byte
// array will be the original json text.
type LiteralServiceAccountKey []byte

func (k *LiteralServiceAccountKey) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.New("UnmarshalJSON on nil pointer")
	}

	// First we try to parse the data as yaml/json
	tmp := make(map[string]interface{})
	err := json.Unmarshal(data, &tmp)
	if err == nil {
		*k = append((*k)[0:0], data...)
		return nil
	}

	return errors.New("value is not valid json")
}

// EncodedServiceAccountKey is a byte array type that can hold a base64-encoded json structure.
// It validates that its value is valid base64-encoded json upon parsing. Upon parsing, the contents
// of the byte array will be the json text after base64 decoding is performed.
type EncodedServiceAccountKey []byte

// UnmarshalJSON sets *m to a copy of data.
func (k *EncodedServiceAccountKey) UnmarshalJSON(data []byte) error {
	if k == nil {
		return errors.New("EncodedServiceAccountKey.UnmarshalJSON: nil pointer")
	}

	var decoded []byte
	var encodedStr string

	// First we decode the data into a string to get rid of any start and end quotes.
	err := yaml.Unmarshal(data, &encodedStr)
	if err != nil {
		return errors.New("EncodedServiceAccountKey.UnmarshalJSON: not a string value")
	}

	decoded, err = base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return errors.New("EncodedServiceAccountKey.UnmarshalJSON: not a valid base64 value")
	}

	// Make sure what we just decoded is valid json
	tmp := make(map[string]interface{})
	err = json.Unmarshal(decoded, &tmp)
	if err != nil {
		return errors.New("EncodedServiceAccountKey.UnmarshalJSON: decoded value is not valid json")
	}

	*k = append((*k)[0:0], decoded...)
	return nil
}

// Config contains configuration for the agent.
type Config struct {
	Identity  *Identity  `json:"identity"`
	Metrics   *Metrics   `json:"metrics"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Identity contains configuration pertaining to the agent identity and credentials.
// Exactly one of the ServiceAccountKey fields must be specified.
type Identity struct {
	ServiceAccountKey        LiteralServiceAccountKey `json:"serviceAccountKey"`
	EncodedServiceAccountKey EncodedServiceAccountKey `json:"encodedServiceAccountKey"`
}

// Metrics contains the metric definitions that the agent expects to receive.
type Metrics struct {
	// The number of seconds that metrics should be aggregated prior to forwarding
	BufferSeconds int64

	// The list of reportable metrics
	Definitions []MetricDefinition `json:"definitions"`

	// Private cache of definitions by name for faster lookup.
	initOnce          sync.Once
	definitionsByName map[string]*MetricDefinition
}

// MetricDefinition describes a single reportable metric's name and type.
type MetricDefinition struct {
	Name        string
	BillingName string
	Type        string
}

// Endpoint describes a single remote endpoint used for sending aggregated metrics.
type Endpoint struct {
	Name           string                  `json:"name"`
	Disk           *DiskEndpoint           `json:"disk"`
	ServiceControl *ServiceControlEndpoint `json:"servicecontrol"`
	PubSub         *PubSubEndpoint         `json:"pubsub"`
}

type DiskEndpoint struct {
	ReportDir     string `json:"reportDir"`
	ExpireSeconds int64  `json:"expireSeconds"`
}

type ServiceControlEndpoint struct {
	ServiceName string `json:"serviceName"`
	ConsumerId  string `json:"consumerId"`
}

type PubSubEndpoint struct {
	Topic string `json:"topic"`
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

// GetMetricDefinition returns the MetricDefinition with the given name, or nil if it does not
// exist.
func (c *Metrics) GetMetricDefinition(name string) *MetricDefinition {
	c.initOnce.Do(func() {
		c.definitionsByName = make(map[string]*MetricDefinition)
		for i := range c.Definitions {
			def := &c.Definitions[i]
			c.definitionsByName[def.Name] = def
		}
	})
	return c.definitionsByName[name]
}

// Validation

type Validatable interface {
	Validate() error
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

func (c *Identity) Validate() error {
	count := 0
	if len(c.ServiceAccountKey) > 0 {
		count += 1
	}
	if len(c.EncodedServiceAccountKey) > 0 {
		count += 1
	}

	if count == 0 {
		return errors.New("identity: missing service account key")
	}
	if count > 1 {
		return errors.New("identity: too many service account keys")
	}

	return nil
}

func (c *Identity) GetServiceAccountKey() []byte {
	if len(c.ServiceAccountKey) > 0 {
		return c.ServiceAccountKey
	}

	if len(c.EncodedServiceAccountKey) > 0 {
		return c.EncodedServiceAccountKey
	}

	return nil
}

// Validate checks validity of metric configuration. Specifically, it must not contain duplicate
// metric definitions, and metric definitions must specify valid type names.
func (c *Metrics) Validate() error {
	usedNames := make(map[string]bool)
	for _, def := range c.Definitions {
		if def.Name == "" {
			return errors.New("missing metric name")
		}
		if def.BillingName == "" {
			return errors.New(fmt.Sprintf("metric %v: missing billing name", def.Name))
		}
		if usedNames[def.Name] {
			return errors.New(fmt.Sprintf("metric %v: duplicate name: %v", def.Name, def.Name))
		}
		usedNames[def.Name] = true
		if def.Type != IntType && def.Type != DoubleType {
			return errors.New(fmt.Sprintf("metric %s: invalid type: %v", def.Name, def.Type))
		}
	}
	return nil
}

func (e *Endpoint) Validate() error {
	if e.Name == "" {
		return errors.New("endpoint: missing name")
	}
	// TODO(volkman): determine other Name requirements (no '/'?)

	types := 0
	for _, v := range []Validatable{e.Disk, e.PubSub, e.ServiceControl} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(); err != nil {
			return err
		}
		types++
	}

	if types == 0 {
		return errors.New(fmt.Sprintf("endpoint %v: missing type configuration", e.Name))
	}

	if types > 1 {
		return errors.New(fmt.Sprintf("endpoint %v: multiple type configurations", e.Name))
	}

	return nil
}

func (e *DiskEndpoint) Validate() error {
	if e.ExpireSeconds < 0 {
		return errors.New("disk: expireSeconds must not be negative")
	}
	if e.ReportDir == "" {
		return errors.New("disk: missing report directory")
	}
	return nil
}

func (e *PubSubEndpoint) Validate() error {
	// TODO(volkman): implement
	return nil
}

func (e *ServiceControlEndpoint) Validate() error {
	if e.ServiceName == "" {
		return errors.New("servicecontrol: missing service name")
	}
	if e.ConsumerId == "" {
		return errors.New("servicecontrol: missing consumer ID")
	}
	if !(strings.HasPrefix(e.ConsumerId, "project:") ||
		strings.HasPrefix(e.ConsumerId, "project_number:") ||
		strings.HasPrefix(e.ConsumerId, "apiKey:")) {
		return errors.New(`servicecontrol: invalid consumer ID (must start with "project:", "projectNumber:", or "apiKey:")`)
	}
	return nil
}
