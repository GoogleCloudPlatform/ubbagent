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
	"strings"
)

// Type Endpoints is a Validatable collection of Endpoint objects.
type Endpoints []Endpoint

func (endpoints Endpoints) Validate(c *Config) error {
	usedNames := make(map[string]bool)
	for _, e := range endpoints {
		if usedNames[e.Name] {
			return fmt.Errorf("endpoint %v: multiple endpoints with the same name", e.Name)
		}
		if err := e.Validate(c); err != nil {
			return err
		}
		usedNames[e.Name] = true
	}
	return nil
}

// Returns whether an endpoint with the given name exists.
func (endpoints Endpoints) exists(name string) bool {
	for _, v := range endpoints {
		if v.Name == name {
			return true
		}
	}
	return false
}

// Endpoint describes a single remote endpoint used for sending aggregated metrics.
type Endpoint struct {
	Name           string                  `json:"name"`
	Disk           *DiskEndpoint           `json:"disk"`
	ServiceControl *ServiceControlEndpoint `json:"servicecontrol"`
	PubSub         *PubSubEndpoint         `json:"pubsub"`
}

func (e *Endpoint) Validate(c *Config) error {
	if e.Name == "" {
		return errors.New("endpoint: missing name")
	}
	// TODO(volkman): determine other Name requirements (no '/'?)

	types := 0
	for _, v := range []Validatable{e.Disk, e.PubSub, e.ServiceControl} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(c); err != nil {
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

type DiskEndpoint struct {
	ReportDir     string `json:"reportDir"`
	ExpireSeconds int64  `json:"expireSeconds"`
}

func (e *DiskEndpoint) Validate(c *Config) error {
	if e.ExpireSeconds < 0 {
		return errors.New("disk: expireSeconds must not be negative")
	}
	if e.ReportDir == "" {
		return errors.New("disk: missing report directory")
	}
	return nil
}

type ServiceControlEndpoint struct {
	Identity    string `json:"identity"`
	ServiceName string `json:"serviceName"`
	ConsumerId  string `json:"consumerId"`
}

func (e *ServiceControlEndpoint) Validate(c *Config) error {
	if err := validateGcpKey(c.Identities, "servicecontrol", e.Identity); err != nil {
		return err
	}
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

type PubSubEndpoint struct {
	Identity string `json:"identity"`
	Topic    string `json:"topic"`
}

func (e *PubSubEndpoint) Validate(c *Config) error {
	// TODO(volkman): implement
	return nil
}

func validateGcpKey(identities Identities, endpointType, identity string) error {
	if identity == "" {
		return fmt.Errorf("%v: missing identity name", endpointType)
	}
	i := identities.Get(identity)
	if i == nil {
		return fmt.Errorf("%v: nonexistent identity: %v", endpointType, identity)
	}
	if i.GCP == nil {
		return fmt.Errorf("%v: %v is not a GCP identity", endpointType, identity)
	}
	return nil
}
