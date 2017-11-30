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

// See Sender.Prepare.
func (d *Dispatcher) Prepare(reports ...metrics.StampedMetricReport) (PreparedSend, error) {
	sends := make([]PreparedSend, len(d.senders))
	for i, s := range d.senders {
		ps, err := s.Prepare(reports...)
		if err != nil {
			return nil, err
		}
		sends[i] = ps
	}
	return &dispatcherSend{d, reports, sends}, nil
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
	for _, s := range d.senders {
		handlers = append(handlers, s.Endpoints()...)
	}
	return
}

type dispatcherSend struct {
	dispatcher *Dispatcher
	reports    []metrics.StampedMetricReport
	sends      []PreparedSend
}

// Send fans out to each PreparedSend in parallel and returns the first error, if any. Send blocks
// until all sub-sends have finished.
func (ds *dispatcherSend) Send() error {
	endpoints := ds.dispatcher.Endpoints()
	for _, r := range ds.reports {
		ds.dispatcher.recorder.Register(r.Id, endpoints...)
	}
	errors := make([]error, len(ds.sends))
	wg := sync.WaitGroup{}
	wg.Add(len(ds.sends))
	for i, ps := range ds.sends {
		go func(i int, ps PreparedSend) {
			errors[i] = ps.Send()
			wg.Done()
		}(i, ps)
	}
	wg.Wait()
	return multierror.Append(nil, errors...).ErrorOrNil()
}

func NewDispatcher(senders []Sender, recorder stats.Recorder) *Dispatcher {
	for _, s := range senders {
		s.Use()
	}
	return &Dispatcher{senders: senders, recorder: recorder}
}
