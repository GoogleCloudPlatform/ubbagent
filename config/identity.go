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
	"encoding/json"
	"encoding/base64"
	"errors"

	"github.com/ghodss/yaml"
)

// Identity holds configuration for identifying to Google Cloud Platform services.
type Identity struct {
	ServiceAccountKey        LiteralServiceAccountKey `json:"serviceAccountKey"`
	EncodedServiceAccountKey EncodedServiceAccountKey `json:"encodedServiceAccountKey"`
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

