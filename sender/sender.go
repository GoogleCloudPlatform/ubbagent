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

package sender

import (
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
)

// PreparedSend is returned by Sender.Prepare() and is used to execute the actual send.
type PreparedSend interface {
	// Send sends an already-prepared report. This method can still generate an error due to
	// unforeseen transient problems (such as network or persistence problems).
	Send() error
}

// Sender handles sending StampedMetricReport objects to remote endpoints.
// The Sender interface is split into a prepare step and a send step, similar to a two-phase commit.
// Sending reports to remote endpoints can involve a pre-processing step which might fail. When
// fanning out to multiple endpoints, preprocessing errors can be caught prior to actually sending.
type Sender interface {
	// Sender is a pipeline.Component.
	pipeline.Component

	// Prepare prepares one or more reports for sending, and returns a Sender used to execute the
	// send. Any failure during the preparation step will be returned as an error.
	Prepare(reports ...metrics.StampedMetricReport) (PreparedSend, error)

	// Endpoints returns the transitive list of endpoints that this sender will ultimately send to.
	Endpoints() []string
}
