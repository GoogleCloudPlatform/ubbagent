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
)

// Build builds pipeline containing a configured Aggregator and all of the resources
// (persistence, endpoints) behind it. It returns the pipeline.Head.
func Build(cfg *config.Config, p persistence.Persistence) (pipeline.Head, error) {
	agentId, err := agentid.CreateOrGet(p)
	if err != nil {
		return nil, err
	}
	endpoints, err := createEndpoints(cfg, agentId)
	if err != nil {
		return nil, err
	}
	senders := make([]sender.Sender, len(endpoints))
	for i := range endpoints {
		senders[i] = sender.NewRetryingSender(endpoints[i], p)
	}
	d := sender.NewDispatcher(senders)

	return aggregator.NewAggregator(cfg.Metrics, d, p), nil
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
			config.Identity.ServiceAccountKey,
		)
	}
	// TODO(volkman): support pubsub
	return nil, errors.New("unsupported endpoint")
}
