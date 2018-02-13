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
	"errors"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/agentid"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint/disk"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint/servicecontrol"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline/inputs"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline/senders"
	"github.com/GoogleCloudPlatform/ubbagent/sources"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/hashicorp/go-multierror"
)

// Build builds pipeline containing a configured Aggregator and all of the resources
// (persistence, endpoints) behind it. It returns the pipeline.Input.
func Build(cfg *config.Config, p persistence.Persistence, r stats.Recorder) (pipeline.Input, error) {
	agentId, err := agentid.CreateOrGet(p)
	if err != nil {
		return nil, err
	}
	endpoints, err := createEndpoints(cfg, agentId)
	if err != nil {
		return nil, err
	}
	endpointSenders := make(map[string]pipeline.Sender)
	for i := range endpoints {
		endpointSenders[endpoints[i].Name()] = senders.NewRetryingSender(endpoints[i], p, r)
	}

	// Inputs for the resultant Selector.
	selectorInputs := make(map[string]pipeline.Input)
	for _, metric := range cfg.Metrics {
		var msenders []pipeline.Sender
		for _, me := range metric.Endpoints {
			msenders = append(msenders, endpointSenders[me.Name])
		}
		di := &pipeline.InputAdapter{Sender: senders.NewDispatcher(msenders, r)}
		if metric.Aggregation != nil {
			bufferTime := time.Duration(metric.Aggregation.BufferSeconds) * time.Second
			selectorInputs[metric.Name] = inputs.NewAggregator(metric.Definition, bufferTime, di, p)
		} else if metric.Passthrough != nil {
			selectorInputs[metric.Name] = di
		}
	}
	selector := inputs.NewSelector(selectorInputs)

	// Defined metric sources.
	var sourcesList []pipeline.Source
	for _, src := range cfg.Sources {
		if src.Heartbeat != nil {
			sourcesList = append(sourcesList, sources.NewHeartbeat(*src.Heartbeat, selector))
		}
	}

	cb := func() error {
		var err *multierror.Error
		for _, src := range sourcesList {
			err = multierror.Append(err, src.Shutdown())
		}
		return err.ErrorOrNil()
	}

	return inputs.NewCallbackInput(selector, cb), nil
}

func createEndpoints(config *config.Config, agentId string) ([]pipeline.Endpoint, error) {
	var eps []pipeline.Endpoint
	for _, cfgep := range config.Endpoints {
		ep, err := createEndpoint(config, &cfgep, agentId)
		if err != nil {
			// TODO(volkman): close already-created endpoints in event of error?
			return nil, err
		}
		eps = append(eps, ep)
	}
	return eps, nil
}

func createEndpoint(config *config.Config, cfgep *config.Endpoint, agentId string) (pipeline.Endpoint, error) {
	if cfgep.Disk != nil {
		return disk.NewDiskEndpoint(
			cfgep.Name,
			cfgep.Disk.ReportDir,
			time.Duration(cfgep.Disk.ExpireSeconds)*time.Second,
		), nil
	}
	if cfgep.ServiceControl != nil {
		return servicecontrol.NewServiceControlEndpoint(
			cfgep.Name,
			cfgep.ServiceControl.ServiceName,
			agentId,
			cfgep.ServiceControl.ConsumerId,
			config.Identities.Get(cfgep.ServiceControl.Identity).GCP.GetServiceAccountKey(),
		)
	}
	// TODO(volkman): support pubsub
	return nil, errors.New("unsupported endpoint")
}
