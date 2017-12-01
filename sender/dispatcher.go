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
	"sync"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/stats"
	"github.com/hashicorp/go-multierror"
)

// Dispatcher is a Sender that fans out to other Sender instances. Generally,
// this will be a collection of Endpoints wrapped in RetryingSender objects.
type Dispatcher struct {
	senders  []Sender
	tracker  pipeline.UsageTracker
	recorder stats.Recorder
}

// Send fans out to each Sender in parallel and returns any errors. Send blocks
// until all sub-sends have finished.
func (d *Dispatcher) Send(report metrics.StampedMetricReport) error {

	// First, register that each report will be handled by this Dispatcher's endpoints.
	endpoints := d.Endpoints()
	d.recorder.Register(report.Id, endpoints)

	// Next, forward the reports to each subsequent sender.
	errors := make([]error, len(d.senders))
	wg := sync.WaitGroup{}
	wg.Add(len(d.senders))
	for i, ps := range d.senders {
		go func(i int, s Sender) {
			// If the send generates an error, we assume that the downstream sender will register that
			// error with the stats recorder.
			errors[i] = s.Send(report)
			wg.Done()
		}(i, ps)
	}
	wg.Wait()
	return multierror.Append(nil, errors...).ErrorOrNil()
}

// Use increments the Dispatcher's usage count.
// See pipeline.Component.Use.
func (d *Dispatcher) Use() {
	d.tracker.Use()
}

// Release decrements the Dispatcher's usage count. If it reaches 0, Release releases all of the
// underlying senders concurrently and waits for the operations to finish.
// See pipeline.Component.Release.
func (d *Dispatcher) Release() error {
	return d.tracker.Release(func() error {
		errors := make([]error, len(d.senders))
		wg := sync.WaitGroup{}
		wg.Add(len(d.senders))
		for i, s := range d.senders {
			go func(i int, s Sender) {
				errors[i] = s.Release()
				wg.Done()
			}(i, s)
		}
		wg.Wait()
		return multierror.Append(nil, errors...).ErrorOrNil()
	})
}

func (d *Dispatcher) Endpoints() (handlers []string) {
	seen := make(map[string]bool)
	for _, s := range d.senders {
		for _, e := range s.Endpoints() {
			if _, exists := seen[e]; !exists {
				seen[e] = true
				handlers = append(handlers, e)
			}
		}
	}
	return
}

func NewDispatcher(senders []Sender, recorder stats.Recorder) *Dispatcher {
	for _, s := range senders {
		s.Use()
	}
	return &Dispatcher{senders: senders, recorder: recorder}
}
