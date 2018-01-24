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

package metrics

import (
	"errors"
	"fmt"
)

const (
	IntType    = "int"
	DoubleType = "double"
)

// Definition describes a single reportable metric's name and type.
type Definition struct {
	Name string
	Type string
}

func (m *Definition) Validate() error {
	if m.Name == "" {
		return errors.New("missing metric name")
	}
	if m.Type != IntType && m.Type != DoubleType {
		return fmt.Errorf("metric %v: invalid value type: %v", m.Name, m.Type)
	}
	return nil
}
