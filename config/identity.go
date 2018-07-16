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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
)

// Type Identities is a Validatable collection of Identity objects.
type Identities []Identity

func (identities Identities) Validate(c *Config) error {
	usedNames := make(map[string]bool)
	for _, i := range identities {
		if usedNames[i.Name] {
			return fmt.Errorf("identity %v: multiple identities with the same name", i.Name)
		}
		if err := i.Validate(c); err != nil {
			return err
		}
		usedNames[i.Name] = true
	}
	return nil
}

func (identities Identities) Get(name string) *Identity {
	for i := range identities {
		if identities[i].Name == name {
			return &identities[i]
		}
	}
	return nil
}

type Identity struct {
	Name string       `json:"name"`
	GCP  *GCPIdentity `json:"gcp"`
}

func (i *Identity) Validate(c *Config) error {
	if i.Name == "" {
		return errors.New("identity: missing name")
	}

	types := 0
	for _, v := range []Validatable{i.GCP} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(c); err != nil {
			return err
		}
		types++
	}

	if types == 0 {
		return fmt.Errorf("identity %v: missing type configuration", i.Name)
	}

	if types > 1 {
		return fmt.Errorf("identity %v: multiple type configurations", i.Name)
	}

	return nil
}

// GCPIdentity holds configuration for identifying to Google Cloud Platform services.
type GCPIdentity struct {
	ServiceAccountKey        *LiteralServiceAccountKey `json:"serviceAccountKey"`
	EncodedServiceAccountKey *EncodedServiceAccountKey `json:"encodedServiceAccountKey"`
}

func (c *GCPIdentity) GetServiceAccountKey() []byte {
	if c.ServiceAccountKey != nil {
		return *c.ServiceAccountKey
	}

	if c.EncodedServiceAccountKey != nil {
		return *c.EncodedServiceAccountKey
	}

	return nil
}

func (i *GCPIdentity) Validate(c *Config) error {
	count := 0
	if i.ServiceAccountKey != nil {
		count += 1
	}
	if i.EncodedServiceAccountKey != nil {
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
