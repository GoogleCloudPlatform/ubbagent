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
	"github.com/GoogleCloudPlatform/ubbagent/aggregator"
	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint/disk"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint/servicecontrol"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/sender"
	"github.com/GoogleCloudPlatform/ubbagent/source"
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
	senders := make(map[string]sender.Sender)
	for i := range endpoints {
		senders[endpoints[i].Name()] = sender.NewRetryingSender(endpoints[i], p, r)
	}

	// Inputs for the resultant Selector.
	inputs := make(map[string]pipeline.Input)
	for _, metric := range cfg.Metrics {
		var msenders []sender.Sender
		for _, me := range metric.Endpoints {
			msenders = append(msenders, senders[me.Name])
		}
		di := &sender.InputAdapter{Sender: sender.NewDispatcher(msenders, r)}
		if metric.Aggregation != nil {
			bufferTime := time.Duration(metric.Aggregation.BufferSeconds) * time.Second
			inputs[metric.Name] = aggregator.NewAggregator(metric.Definition, bufferTime, di, p)
		} else if metric.Passthrough != nil {
			inputs[metric.Name] = di
		}
	}
	selector := pipeline.NewSelector(inputs)

	// Defined metric sources.
	var sources []pipeline.Source
	for _, src := range cfg.Sources {
		if src.Heartbeat != nil {
			sources = append(sources, source.NewHeartbeat(*src.Heartbeat, selector))
		}
	}

	cb := func() error {
		var err *multierror.Error
		for _, src := range sources {
			err = multierror.Append(err, src.Shutdown())
		}
		return err.ErrorOrNil()
	}

	return pipeline.NewCallbackInput(selector, cb), nil
}

func createEndpoints(config *config.Config, agentId string) ([]endpoint.Endpoint, error) {
	var eps []endpoint.Endpoint
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

func createEndpoint(config *config.Config, cfgep *config.Endpoint, agentId string) (endpoint.Endpoint, error) {
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
