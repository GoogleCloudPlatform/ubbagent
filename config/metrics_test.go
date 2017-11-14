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

package config_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/ubbagent/config"
)

func TestMetrics_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		validConfig := config.Metrics{
			Definitions: []config.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric2", Type: "int"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := validConfig.Validate(nil)
		if err != nil {
			t.Fatalf("Expected no error, got: %s", err)
		}
	})

	t.Run("invalid: duplicate metric", func(t *testing.T) {
		duplicateName := config.Metrics{
			Definitions: []config.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric", Type: "int"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := duplicateName.Validate(nil)
		if err == nil || err.Error() != "metric int-metric: duplicate name: int-metric" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: invalid type", func(t *testing.T) {
		invalidType := config.Metrics{
			Definitions: []config.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric2", Type: "foo"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := invalidType.Validate(nil)
		if err == nil || err.Error() != "metric int-metric2: invalid type: foo" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})
}

func TestMetrics_GetMetricDefinition(t *testing.T) {
	validConfig := config.Metrics{
		Definitions: []config.MetricDefinition{
			{Name: "int-metric", Type: "int"},
			{Name: "int-metric2", Type: "int"},
			{Name: "double-metric", Type: "double"},
		},
	}

	expected := config.MetricDefinition{
		Name:        "int-metric2",
		Type:        "int",
	}
	actual := validConfig.GetMetricDefinition("int-metric2")
	if *actual != expected {
		t.Fatalf("Expected: %s, got: %s", expected, actual)
	}

	actual = validConfig.GetMetricDefinition("bogus")
	if actual != nil {
		t.Fatalf("Expected: nil, got: %s", actual)
	}
}
