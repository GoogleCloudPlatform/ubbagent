// Copyright 2018 Google LLC
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

package testlib

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

// Type waitForCalls is a base type that provides a doAndWait function.
type waitForCalls struct {
	calls    int32
	waitChan chan bool
}

// DoAndWait executes the given function and then waits until the total number of calls reaches the
// given value.
func (wfc *waitForCalls) DoAndWait(t *testing.T, calls int32, f func()) {
	f()
	for atomic.LoadInt32(&wfc.calls) < calls {
		select {
		case <-wfc.waitChan:
		case <-time.After(5 * time.Second):
			t.Fatal("DoAndWait: nothing happened after 5 seconds")
		}
	}
}

func (wfc *waitForCalls) called() {
	atomic.AddInt32(&wfc.calls, 1)
	wfc.waitChan <- true
}

func (wfc *waitForCalls) Calls() int32 {
	return atomic.LoadInt32(&wfc.calls)
}

func (wfc *waitForCalls) wfcInit() {
	wfc.waitChan = make(chan bool, 100)
}

type MockSender struct {
	waitForCalls
	Used     bool
	Released bool

	reports   []metrics.MetricReport // must hold mu to read/write
	sendErr   error
	mu        sync.Mutex
	endpoints []string
}

func (s *MockSender) Send(report metrics.StampedMetricReport) error {
	s.mu.Lock()
	err := s.sendErr
	if err == nil {
		s.reports = append(s.reports, report.MetricReport)
	}
	s.mu.Unlock()
	s.called()
	return err
}

func (s *MockSender) Endpoints() []string {
	return s.endpoints
}

func (s *MockSender) Use() {
	s.Used = true
}

func (s *MockSender) Release() error {
	s.Released = true
	return nil
}

func (s *MockSender) Reports() (reports []metrics.MetricReport) {
	s.mu.Lock()
	reports = s.reports
	s.reports = []metrics.MetricReport{}
	s.mu.Unlock()
	return
}

func (s *MockSender) SetSendError(err error) {
	s.sendErr = err
}

// NewMockSender creates a new MockSender with the given endpoint IDs.
func NewMockSender(endpoints ...string) *MockSender {
	ms := &MockSender{endpoints: endpoints}
	ms.Reports()
	ms.wfcInit()
	return ms
}

// Type MockEndpoint is a mock endpoint.Endpoint.
type MockEndpoint struct {
	waitForCalls
	Used     bool
	Released bool

	reports  []endpoint.EndpointReport // must hold mu to read/write
	name     string
	sendErr  error
	buildErr error
	mu       sync.Mutex
}

func (ep *MockEndpoint) Name() string {
	return ep.name
}

func (ep *MockEndpoint) Send(report endpoint.EndpointReport) error {
	ep.mu.Lock()
	err := ep.sendErr
	if err == nil {
		ep.reports = append(ep.reports, report)
	}
	ep.mu.Unlock()
	ep.called()
	return err
}

func (ep *MockEndpoint) BuildReport(report metrics.StampedMetricReport) (endpoint.EndpointReport, error) {
	if ep.buildErr != nil {
		return endpoint.EndpointReport{}, ep.buildErr
	}
	return endpoint.NewEndpointReport(report, nil)
}

func (ep *MockEndpoint) Use() {
	ep.Used = true
}

func (ep *MockEndpoint) Release() error {
	ep.Released = true
	return nil
}

func (ep *MockEndpoint) IsTransient(err error) bool {
	return err != nil && err.Error() != "FATAL"
}

func (ep *MockEndpoint) Reports() (reports []endpoint.EndpointReport) {
	ep.mu.Lock()
	reports = ep.reports
	ep.reports = []endpoint.EndpointReport{}
	ep.mu.Unlock()
	return
}

func (ep *MockEndpoint) SetSendErr(err error) {
	ep.mu.Lock()
	ep.sendErr = err
	ep.mu.Unlock()
}

func (ep *MockEndpoint) SetBuildErr(err error) {
	ep.mu.Lock()
	ep.buildErr = err
	ep.mu.Unlock()
}

// NewMockEndpoint creates a new MockEndpoint with the given name.
func NewMockEndpoint(name string) *MockEndpoint {
	ep := &MockEndpoint{name: name}
	ep.Reports()
	ep.wfcInit()
	return ep
}

// Type MockStatsRecorder is a mock stats.StatsRecorder.
type MockStatsRecorder struct {
	waitForCalls
	mu         sync.RWMutex
	registered map[string][]string
	succeeded  []RecordedEntry
	failed     []RecordedEntry
}

type RecordedEntry struct {
	Id      string
	Handler string
}

func (sr *MockStatsRecorder) Register(id string, handlers []string) {
	sr.mu.Lock()
	if sr.registered == nil {
		sr.registered = make(map[string][]string)
	}
	sr.registered[id] = handlers
	sr.mu.Unlock()
}

func (sr *MockStatsRecorder) SendSucceeded(id string, handler string) {
	sr.mu.Lock()
	sr.succeeded = append(sr.succeeded, RecordedEntry{id, handler})
	sr.mu.Unlock()
	sr.called()
}

func (sr *MockStatsRecorder) SendFailed(id string, handler string) {
	sr.mu.Lock()
	sr.failed = append(sr.failed, RecordedEntry{id, handler})
	sr.mu.Unlock()
	sr.called()
}

func (sr *MockStatsRecorder) Registered() map[string][]string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.registered
}

func (sr *MockStatsRecorder) Succeeded() []RecordedEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.succeeded
}

func (sr *MockStatsRecorder) Failed() []RecordedEntry {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.failed
}

func NewMockStatsRecorder() *MockStatsRecorder {
	sr := &MockStatsRecorder{}
	sr.wfcInit()
	return sr
}
