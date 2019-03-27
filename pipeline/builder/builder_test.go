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

package builder

import (
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
)

// TestBuild tests that a Pipeline can be created and shutdown successfully.
func TestBuild(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "build_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)
	p, err := persistence.NewDiskPersistence(filepath.Join(tmpdir, "state"))
	if err != nil {
		t.Fatalf("Unable to create disk persistence: %+v", err)
	}

	cfg := &config.Config{
		Metrics: config.Metrics{
			{
				Definition: metrics.Definition{
					Name: "int-metric",
					Type: "int",
				},
				Aggregation: &config.Aggregation{
					BufferSeconds: 10,
				},
				Endpoints: []config.MetricEndpoint{
					{Name: "on_disk"},
				},
			},
			{
				Definition: metrics.Definition{
					Name: "double-metric",
					Type: "double",
				},
				Aggregation: &config.Aggregation{
					BufferSeconds: 10,
				},
				Endpoints: []config.MetricEndpoint{
					{Name: "on_disk"},
				},
			},
		},
		Endpoints: []config.Endpoint{
			{
				Name: "on_disk",
				Disk: &config.DiskEndpoint{
					ReportDir:     filepath.Join(tmpdir, "reports"),
					ExpireSeconds: 3600,
				},
			},
		},
		Sources: []config.Source{
			{
				Name: "instance-seconds",
				Heartbeat: &config.Heartbeat{
					Metric:          "int-metric",
					IntervalSeconds: 10,
					Value: metrics.MetricValue{
						Int64Value: testlib.Int64Ptr(10),
					},
					Labels: map[string]string{"foo": "bar"},
				},
			},
		},
	}

	a, err := Build(cfg, p, stats.NewNoopRecorder())
	if err != nil {
		t.Fatalf("unexpected error creating App: %+v", err)
	}

	a.Release()
}
