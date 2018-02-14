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
	"reflect"
)

func TestFilters_Validate(t *testing.T) {
	conf := config.Config{
		Metrics: config.Metrics{
			{
				Definition:  metrics.Definition{Name: "int-metric", Type: "int"},
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

	t.Run("valid", func(t *testing.T) {
		c := conf
		c.Filters = config.Filters{
			{
				AddLabels: &config.AddLabels{
					Labels: map[string]string{
						"foo1": "bar1",
						"foo2": "bar2",
					},
				},
			},
		}

		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected validate error: %v", err)
		}
	})

	t.Run("invalid: missing filter config", func(t *testing.T) {
		c := conf
		c.Filters = config.Filters{
			{},
		}

		expected := "missing filter configuration"
		if err := c.Validate(); err == nil {
			t.Fatal("expected validate error, got nil")
		} else if err.Error() != expected {
			t.Fatalf("validate error: want=%v, got=%v", expected, err.Error())
		}
	})

	t.Run("invalid: missing labels", func(t *testing.T) {
		c := conf
		c.Filters = config.Filters{
			{
				AddLabels: &config.AddLabels{},
			},
		}

		expected := "addLabels: missing labels"
		if err := c.Validate(); err == nil {
			t.Fatal("expected validate error, got nil")
		} else if err.Error() != expected {
			t.Fatalf("validate error: want=%v, got=%v", expected, err.Error())
		}
	})
}

func TestAddLabels_IncludedLabels(t *testing.T) {
	t.Run("empty labels are omitted", func(t *testing.T) {
		a := config.AddLabels{
			OmitEmpty: true,
			Labels: map[string]string{
				"foo":   "bar",
				"bar":   "baz",
				"empty": "",
			},
		}
		expected := map[string]string{
			"foo": "bar",
			"bar": "baz",
		}
		if want, got := expected, a.IncludedLabels(); !reflect.DeepEqual(want, got) {
			t.Fatalf("IncludedLabels(): want: %+v, got: %+v", want, got)
		}
	})

	t.Run("empty labels are included", func(t *testing.T) {
		a := config.AddLabels{
			OmitEmpty: false,
			Labels: map[string]string{
				"foo":   "bar",
				"bar":   "baz",
				"empty": "",
			},
		}
		expected := map[string]string{
			"foo":   "bar",
			"bar":   "baz",
			"empty": "",
		}
		if want, got := expected, a.IncludedLabels(); !reflect.DeepEqual(want, got) {
			t.Fatalf("IncludedLabels(): want: %+v, got: %+v", want, got)
		}
	})
}
