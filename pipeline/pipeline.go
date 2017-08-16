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

// Package pipeline describes a metrics reporting pipeline that accepts reports as input and
// eventually delivers them (likely after aggregation) to one or more downstream services.
// A fully-constructed pipeline consists of an aggregator, a dispatcher, and one or more endpoints
// wrapped in RetryingSender objects:
//
//                          -> RetryingSender -> Endpoint A
// Aggregator -> Dispatcher -> RetryingSender -> Endpoint B
//                          -> RetryingSender -> Endpoint C
//
package pipeline

import "github.com/GoogleCloudPlatform/ubbagent/metrics"

// Head represents the start of a pipeline. It is a Component that accepts metric reports as input.
type Head interface {
	// Head is also a Component.
	Component

	// AddReport adds a report to the pipeline. It returns an error if one is known immediately,
	// such as a report that refers to unknown metrics. See aggregator.Aggregator.
	AddReport(metrics.MetricReport) error
}

// Component represents a single component in a pipeline that can be closed.
type Component interface {
	// Close closes this Component. Close must perform the following steps, in order:
	// 1. Gracefully shutdown background processes and wait for completion. Following this step,
	//    no data shall be sent from this component to downstream components.
	// 2. Call Close on all adjacent components, and wait for their close operations to
	//    complete.
	//
	// As a result, calling Close on the outer Pipeline should result in a graceful shutdown of
	// all Components in the correct order.
	//
	// Close returns an error if it or any of the descendant components generate one.
	Close() error
}
