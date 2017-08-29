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

package endpoint

import (
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
)

// EndpointReport is an Endpoint-specific structure that contains a MetricBatch formatted for
// consumption by the remote service represented by the Endpoint. A report may contain additional
// information, such as a unique ID used for deduplication. The Dispatcher handles send failure
// retries, and may call the Endpoint's Send method multiple times with the same Report instance.
type EndpointReport interface{}

// Endpoint represents a metric reporting endpoint that the agent reports to. For example, Cloud
// Service Control or PubSub.
type Endpoint interface {
	// Endpoint is a pipeline.Component.
	pipeline.Component

	// Name returns the name of this endpoint. The name must be unique across all endpoints in the
	// system, and should be constant across restarts of the agent. There can be multiple instances
	// of the same type of endpoint with different names.
	Name() string

	// Send sends an EndpointReport previously built by this Endpoint.
	Send(EndpointReport) error

	// BuildReport builds an EndpointReport from the given MetricBatch. The contents of the report
	// are specific to the endpoint.
	BuildReport(metrics.MetricBatch) (EndpointReport, error)

	// EmptyReport returns a pointer to an empty EndpointReport structure and is used when loading
	// previously serialized reports from persistent state.
	EmptyReport() EndpointReport
}
