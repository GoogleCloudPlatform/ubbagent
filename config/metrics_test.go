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
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

func TestMetrics_Validate(t *testing.T) {

	conf := config.Config{
		Endpoints: config.Endpoints{
			{
				Name: "disk1",
				Disk: &config.DiskEndpoint{
					ReportDir:     "/tmp/foo1",
					ExpireSeconds: 3600,
				},
			},
			{
				Name: "disk2",
				Disk: &config.DiskEndpoint{
					ReportDir:     "/tmp/foo2",
					ExpireSeconds: 3600,
				},
			},
		},
	}

	goodAggregation := &config.Aggregation{
		BufferSeconds: 10,
	}
	goodEndpoints := []config.MetricEndpoint{
		{Name: "disk1"},
	}

	t.Run("valid", func(t *testing.T) {
		validConfig := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Endpoints:   goodEndpoints,
				Aggregation: goodAggregation,
			},
			{
				Definition:  metrics.Definition{Name: "int-metric2", Type: "int"},
				Endpoints:   goodEndpoints,
				Aggregation: goodAggregation,
			},
			{
				Definition:  metrics.Definition{Name: "double-metric", Type: "double"},
				Endpoints:   goodEndpoints,
				Aggregation: goodAggregation,
			},
		}

		err := validConfig.Validate(&conf)
		if err != nil {
			t.Fatalf("Expected no error, got: %s", err)
		}
	})

	t.Run("invalid: duplicate metric", func(t *testing.T) {
		duplicateName := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Endpoints:   goodEndpoints,
				Aggregation: goodAggregation,
			},
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Endpoints:   goodEndpoints,
				Aggregation: goodAggregation,
			},
		}

		err := duplicateName.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric: duplicate name: int-metric" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: invalid value type", func(t *testing.T) {
		invalidType := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric2", Type: "foo"},
				Endpoints:   goodEndpoints,
				Aggregation: goodAggregation,
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric2: invalid value type: foo" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: missing buffering configuration", func(t *testing.T) {
		invalidType := config.Metrics{
			{
				Definition: metrics.Definition{Name: "int-metric", Type: "int"},
				Endpoints:  goodEndpoints,
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric: missing buffering configuration" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: no endpoints defined", func(t *testing.T) {
		invalidType := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Aggregation: goodAggregation,
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric: no endpoints defined" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: endpoint missing name", func(t *testing.T) {
		invalidType := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Aggregation: goodAggregation,
				Endpoints: []config.MetricEndpoint{
					{Name: "disk1"},
					{},
				},
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric: endpoint missing name" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: endpoint does not exist", func(t *testing.T) {
		invalidType := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Aggregation: goodAggregation,
				Endpoints: []config.MetricEndpoint{
					{Name: "disk1"},
					{Name: "bogus"},
				},
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric: endpoint does not exist: bogus" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: endpoint listed twice", func(t *testing.T) {
		invalidType := config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Aggregation: goodAggregation,
				Endpoints: []config.MetricEndpoint{
					{Name: "disk1"},
					{Name: "disk2"},
					{Name: "disk2"},
				},
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "metric int-metric: endpoint listed twice: disk2" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("aggregation: bufferSeconds must be > 0", func(t *testing.T) {
		cases := []struct {
			val int64
			msg string
		}{
			{-1, "metric int-metric: bufferSeconds must be > 0"},
			{0, "metric int-metric: bufferSeconds must be > 0"},
			{1, ""},
		}
		for _, c := range cases {
			invalidType := config.Metrics{
				{
					Definition: metrics.Definition{Name: "int-metric", Type: "int"},
					Endpoints:  goodEndpoints,
					Aggregation: &config.Aggregation{
						BufferSeconds: c.val,
					},
				},
			}

			err := invalidType.Validate(&conf)
			if c.msg == "" && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if c.msg != "" && (err == nil || err.Error() != c.msg) {
				t.Fatalf("Expected error, got: %v", err)
			}
		}
	})
}

func TestMetrics_GetMetricDefinition(t *testing.T) {
	validConfig := config.Metrics{
		{Definition: metrics.Definition{Name: "int-metric", Type: "int"}},
		{Definition: metrics.Definition{Name: "int-metric2", Type: "int"}},
		{Definition: metrics.Definition{Name: "double-metric", Type: "double"}},
	}

	expected := metrics.Definition{
		Name: "int-metric2",
		Type: "int",
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
