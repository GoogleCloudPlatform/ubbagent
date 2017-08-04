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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"ubbagent/config"
	"ubbagent/persistence"
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
		Metrics: &config.Metrics{
			BufferSeconds: 10,
			Definitions: []config.MetricDefinition{
				{
					Name: "int-metric",
					Type: "int",
				},
				{
					Name: "double-metric",
					Type: "double",
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
	}

	a, err := Build(cfg, p)
	if err != nil {
		t.Fatalf("unexpected error creating App: %+v", err)
	}

	a.Close()
}
