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
)

type Filter struct {
	// oneof
	AddLabels *AddLabels `json:"addLabels"`
}

func (f *Filter) Validate(c *Config) error {
	types := 0
	for _, v := range []Validatable{f.AddLabels} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(c); err != nil {
			return err
		}
		types++
	}

	if types == 0 {
		return errors.New("missing filter configuration")
	}

	if types > 1 {
		return fmt.Errorf("multiple filter configurations")
	}

	return nil
}

type Filters []Filter

func (m Filters) Validate(c *Config) error {
	for _, def := range m {
		if err := def.Validate(c); err != nil {
			return err
		}
	}
	return nil
}

type AddLabels struct {
	OmitEmpty bool              `json:"omitEmpty"`
	Labels    map[string]string `json:"labels"`
}

func (f *AddLabels) Validate(c *Config) error {
	if len(f.Labels) == 0 {
		return errors.New("addLabels: missing labels")
	}
	return nil
}

// IncludedLabels returns the labels that should be added to input. Empty label values are
// omitted if OmitEmpty is true.
func (f *AddLabels) IncludedLabels() map[string]string {
	included := make(map[string]string)
	for k, v := range f.Labels {
		if v == "" && f.OmitEmpty {
			continue
		}
		included[k] = v
	}
	return included
}
