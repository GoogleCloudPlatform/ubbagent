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
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

// A Sender handles sending StampedMetricReports to remote endpoints.
type Sender interface {
	// Sender is a pipeline.Component.
	Component

	// Send sends the report downstream. The behavior of the Send operation depends on the type of
	// sender. Some implementations - the Dispatcher, for instance - simply forward the Send to
	// subsequent Senders. Others - like the RetryingSender - may queue the report and attempt to
	// send it at a later time.
	//
	// An error indicates that something failed quickly, but it does not
	// indicate that the operation failed completely (i.e., some senders behind a Dispatcher may have
	// succeeded). Likewise, the lack of an error response does not indicate that the Send operation
	// succeeded, due to the asynchronous nature of a RetryingSender.
	Send(report metrics.StampedMetricReport) error

	// Endpoints returns the transitive list of endpoints that this sender will ultimately send to.
	Endpoints() []string
}

// Type InputAdapter is a pipeline.Input that converts incoming reports to StampedMetricReport
// objects and sends them directly to a delegate Sender.
type InputAdapter struct {
	Sender Sender
}

func (a *InputAdapter) AddReport(report metrics.MetricReport) error {
	return a.Sender.Send(metrics.NewStampedMetricReport(report))
}

func (a *InputAdapter) Use() {
	a.Sender.Use()
}

func (a *InputAdapter) Release() error {
	return a.Sender.Release()
}
