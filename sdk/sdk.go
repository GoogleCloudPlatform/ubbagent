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

package sdk

import (
	"encoding/json"

	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline/builder"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
)

// Agent is a convenience type that encapsulates a pipeline.Input and a stats.Provider and provides
// programmatic interfaces similar to those provided by the standalone agent: init, add report,
// get status, shutdown. Agent is used by the various language-specific SDK implementations
// contained under this package.
type Agent struct {
	input    pipeline.Input
	provider stats.Provider
}

// NewAgent creates a new Agent. The configuration is passed as YAML or JSON in configData. The
// state directory is passed as stateDir. If stateDir is empty, state will not be persisted.
func NewAgent(configData []byte, stateDir string) (*Agent, error) {
	cfg, err := parseConfig(configData)
	if err != nil {
		return nil, err
	}

	var p persistence.Persistence
	if stateDir == "" {
		p = persistence.NewMemoryPersistence()
	} else {
		var err error
		p, err = persistence.NewDiskPersistence(stateDir)
		if err != nil {
			return nil, err
		}
	}

	basic := stats.NewBasic()
	input, err := builder.Build(cfg, p, basic)
	if err != nil {
		return nil, err
	}

	return &Agent{input, basic}, nil
}

// Shutdown terminates this agent.
func (agent *Agent) Shutdown() error {
	err := agent.input.Release()
	if err != nil {
		return err
	}
	return nil
}

// AddReport adds a new usage report.
func (agent *Agent) AddReport(report metrics.MetricReport) error {
	return agent.input.AddReport(report.Copy())
}

// AddReportJson adds a new usage report after fist unmarshalling it from JSON.
func (agent *Agent) AddReportJson(reportData []byte) error {
	report, err := ParseReport(reportData)
	if err != nil {
		return err
	}
	return agent.AddReport(report)
}

// GetStatus returns a stats.Snapshot object containing current agent status.
func (agent *Agent) GetStatus() stats.Snapshot {
	return agent.provider.Snapshot()
}

// GetStatusJson returns a stats.Snapshot object serialized as JSON.
func (agent *Agent) GetStatusJson() ([]byte, error) {
	status := agent.GetStatus()
	return SerializeStatus(status)
}

// ParseReport parses the given JSON data and returns a metrics.MetricReport, or an error.
func ParseReport(reportData []byte) (report metrics.MetricReport, err error) {
	err = json.Unmarshal(reportData, &report)
	return
}

// SerializeStatus serializes the given stats.Snapshot in JSON, or returns an error.
func SerializeStatus(status stats.Snapshot) (data []byte, err error) {
	return json.Marshal(status)
}

func parseConfig(configData []byte) (*config.Config, error) {
	cfg, err := config.Parse(configData)
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
