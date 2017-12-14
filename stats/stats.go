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

package stats

import "time"

// A Recorder records the result of sending a metrics.StampedMetricReport to one or more endpoints.
//
// A Recorder expects the following flow:
// 1. The Register method is called prior to performing a send. The method is passed the ID of the
//    StampedMetricReport being sent and a list of the handlers that will perform the operation.
//    Register is called by the first Sender in a pipeline, generally a sender.Dispatcher.
// 2. As each handler succeeds or fails in performing its portion of the overall operation, it
//    registers the result using the SendSucceeded and SendFailed methods. The handlers are
//    generally instances of sender.RetryingSender, wrapping endpoints.
//
// The id value should be set to the value of a StampedMetricReport.Id. A handler should generally
// be set to the name of an endpoint handling part of the send operation.
type Recorder interface {
	Register(id string, handlers []string)
	SendSucceeded(id string, handler string)
	SendFailed(id string, handler string)
}

// A Provider provides recorded stats in the form of a Snapshot.
type Provider interface {
	// Snapshot returns a Snapshot containing current stats.
	Snapshot() Snapshot
}

// Snapshot encapsulates a point-in-time snapshot of agent send stats.
type Snapshot struct {
	// The last time a send succeeded.
	LastReportSuccess time.Time `json:"lastReportSuccess"`

	// The number of failures since the last success.
	CurrentFailureCount int `json:"currentFailureCount"`

	// The number of failures since the last success.
	TotalFailureCount int `json:"totalFailureCount"`
}

// NewNoopRecorder returns a Recorder that does nothing.
func NewNoopRecorder() Recorder {
	return &noopRecorder{}
}

type noopRecorder struct{}

func (*noopRecorder) Register(string, []string)    {}
func (*noopRecorder) SendSucceeded(string, string) {}
func (*noopRecorder) SendFailed(string, string)    {}
