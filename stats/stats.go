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

// A Recorder records the result of sending a metrics.MetricBatch to one or more endpoints.
//
// A Recorder expects the following flow:
// 1. The Register method is called immediately prior to performing a send. The method is passed an
//    ExpectedSend instance, which is likely provided by sender.PreparedSend. The agent's
//    aggregator.Aggregator instance calls Register.
// 2. As each handler succeeds or fails in performing its portion of the overall operation, it
//    registers the result using the SendSucceeded and SendFailed methods. The handlers are most
//    likely instances of sender.RetryingSender, wrapping endpoints.
//
// The batchId value should be set to the value of a MetricsBatch.Id. A handler should most likely
// be set to the name of an endpoint handling part of the send operation.
type Recorder interface {
	Register(send ExpectedSend)
	SendSucceeded(batchId string, handler string)
	SendFailed(batchId string, handler string)
}

// An ExpectedSend represents a report that is about to be sent to 1 or more endpoints. ExpectedSend
// provides both an identifier and a list of handlers that will carry out the send operation. Each
// handler is expected to register its ultimate success or failure using the same identifier.
type ExpectedSend interface {
	BatchId() string
	Handlers() []string
}

type noopRecorder struct{}

// NewNoopRecorder returns a Recorder that does nothing.
func NewNoopRecorder() Recorder {
	return &noopRecorder{}
}

func (*noopRecorder) Register(ExpectedSend)        {}
func (*noopRecorder) SendSucceeded(string, string) {}
func (*noopRecorder) SendFailed(string, string)    {}
