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
