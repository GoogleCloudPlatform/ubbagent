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

package pipeline

import (
	"encoding/json"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

// EndpointReport is a metrics.StampedMetricReport containing optional endpoint-specific context
// that can be used to help ensure idempotence across retries. For example, if a reporting service
// requires a unique ID or timestamp that remains the same during each retry so that requests can
// be deduplicated, that identifier can be generated in BuildReport, persisted in the
// EndpointReport's context, and resent with each retry.
type EndpointReport struct {
	metrics.StampedMetricReport `json:",inline"`
	Context                     json.RawMessage
}

// UnmarshalContext unmarshals an EndpointReport's context into the given struct.
func (er *EndpointReport) UnmarshalContext(ctx interface{}) error {
	return json.Unmarshal(er.Context, ctx)
}

func NewEndpointReport(report metrics.StampedMetricReport, context interface{}) (EndpointReport, error) {
	var msg json.RawMessage
	if context != nil {
		bytes, err := json.Marshal(context)
		if err != nil {
			return EndpointReport{}, err
		}
		msg = json.RawMessage(bytes)
	}
	return EndpointReport{report, msg}, nil
}

// Endpoint represents a metric reporting endpoint that the agent reports to. For example, Cloud
// Service Control or PubSub.
type Endpoint interface {
	// Endpoint is a pipeline.Component.
	Component

	// Name returns the name of this endpoint. The name must be unique across all endpoints in the
	// system, and should be constant across restarts of the agent. There can be multiple instances
	// of the same type of endpoint with different names.
	Name() string

	// Send sends the given EndpointReport - previously built by this endpoint - to the reporting
	// service.
	Send(EndpointReport) error

	// BuildReport builds an EndpointReport from the given StampedMetricReport, optionally attaching
	// context.
	BuildReport(report metrics.StampedMetricReport) (EndpointReport, error)

	// IsTransient returns true if the given error indicates that the operation failed due to some
	// transient error and can be retried.
	IsTransient(error) bool
}
