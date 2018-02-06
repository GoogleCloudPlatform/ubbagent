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

package config_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

func TestSources_Validate(t *testing.T) {
	conf := config.Config{
		Metrics: config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
				Endpoints:   []config.MetricEndpoint{{Name: "disk"}},
				Aggregation: &config.Aggregation{BufferSeconds: 10},
			},
			{
				Definition:  metrics.Definition{Name: "double-metric", Type: "double"},
				Endpoints:   []config.MetricEndpoint{{Name: "disk"}},
				Aggregation: &config.Aggregation{BufferSeconds: 10},
			},
		},
		Endpoints: config.Endpoints{
			{
				Name: "disk",
				Disk: &config.DiskEndpoint{
					ReportDir:     "/tmp/foo1",
					ExpireSeconds: 3600,
				},
			},
		},
	}

	goodHeartbeat := &config.Heartbeat{
		Metric:          "int-metric",
		IntervalSeconds: 10,
		Value: metrics.MetricValue{
			Int64Value: 10,
		},
	}

	t.Run("invalid: duplicate metric", func(t *testing.T) {
		duplicateName := config.Sources{
			{
				Name:      "foo",
				Heartbeat: goodHeartbeat,
			},
			{
				Name:      "foo",
				Heartbeat: goodHeartbeat,
			},
		}

		err := duplicateName.Validate(&conf)
		if err == nil || err.Error() != "source foo: duplicate name: foo" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("heartbeat: intervalSeconds must be > 0", func(t *testing.T) {
		cases := []struct {
			val int64
			msg string
		}{
			{-1, "source test: intervalSeconds must be > 0"},
			{0, "source test: intervalSeconds must be > 0"},
			{1, ""},
		}
		for _, c := range cases {
			invalidType := config.Sources{
				{
					Name: "test",
					Heartbeat: &config.Heartbeat{
						Metric:          "int-metric",
						IntervalSeconds: c.val,
						Value: metrics.MetricValue{
							Int64Value: 10,
						},
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

	t.Run("heartbeat: value must match metric type", func(t *testing.T) {
		validType := config.Source{
			Name: "test",
			Heartbeat: &config.Heartbeat{
				Metric:          "int-metric",
				IntervalSeconds: 10,
				Value: metrics.MetricValue{
					Int64Value: 10,
				},
			},
		}

		if err := validType.Validate(&conf); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		invalidType := config.Source{
			Name: "test",
			Heartbeat: &config.Heartbeat{
				Metric:          "int-metric",
				IntervalSeconds: 10,
				Value: metrics.MetricValue{
					DoubleValue: 10,
				},
			},
		}

		err := invalidType.Validate(&conf)
		if err == nil || err.Error() != "source test: double value specified for integer metric: 10" {
			t.Fatalf("Expected error, got: %v", err)
		}
	})
}
