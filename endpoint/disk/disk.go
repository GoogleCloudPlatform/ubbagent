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

package disk

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/golang/glog"
)

const (
	fileMode        = 0644
	directoryMode   = 0755
	cleanupInterval = 1 * time.Minute
	reportPrefix    = "report_"
	reportSuffix    = ".json"
)

type DiskEndpoint struct {
	name       string
	path       string
	expiration time.Duration
	quit       chan bool
	closeOnce  sync.Once
	clock      clock.Clock
	wait       sync.WaitGroup
	tracker    pipeline.UsageTracker
	closed     bool // used for testing
}

type diskReport struct {
	Name   string
	Report metrics.StampedMetricReport
}

func (r diskReport) Id() string {
	return r.Report.Id
}

// NewDiskEndpoint creates a new DiskEndpoint and starts a goroutine that cleans up expired reports
// on disk.
func NewDiskEndpoint(name, path string, expiration time.Duration) *DiskEndpoint {
	return newDiskEndpoint(name, path, expiration, clock.NewRealClock())
}

func newDiskEndpoint(name, path string, expiration time.Duration, clock clock.Clock) *DiskEndpoint {
	ep := &DiskEndpoint{
		name:       name,
		path:       path,
		expiration: expiration,
		clock:      clock,
		quit:       make(chan bool, 1),
	}
	ep.wait.Add(1)
	go ep.run()
	return ep
}

func (ep *DiskEndpoint) Name() string {
	return ep.name
}

func (ep *DiskEndpoint) Send(report endpoint.EndpointReport) error {
	r := report.(*diskReport)

	jsontext, err := json.Marshal(r.Report)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ep.path, directoryMode); err != nil {
		return err
	}
	file := path.Join(ep.path, r.Name)

	if err := ioutil.WriteFile(file, jsontext, fileMode); err != nil {
		return err
	}
	return nil
}

func (ep *DiskEndpoint) BuildReport(r metrics.StampedMetricReport) (endpoint.EndpointReport, error) {
	return &diskReport{
		Name:   reportName(ep.clock.Now()),
		Report: r,
	}, nil
}

func (*DiskEndpoint) EmptyReport() endpoint.EndpointReport {
	return &diskReport{}
}

// Use increments the DiskEndpoint's usage count.
// See pipeline.Component.Use.
func (ep *DiskEndpoint) Use() {
	ep.tracker.Use()
}

// Release decrements the DiskEndpoint's usage count. If it reaches 0, Release instructs the
// DiskEndpoint's cleanup goroutine to gracefully shutdown. It blocks until the operation has
// completed.
// See pipeline.Component.Release.
func (ep *DiskEndpoint) Release() error {
	return ep.tracker.Release(func() error {
		ep.closeOnce.Do(func() {
			ep.quit <- true
			ep.closed = true
		})
		ep.wait.Wait()
		return nil
	})
}

func (ep *DiskEndpoint) run() {
	for {
		t := ep.clock.NewTimer(cleanupInterval)
		select {
		case <-t.GetC():
			ep.cleanup()
		case <-ep.quit:
			ep.wait.Done()
			return
		}
		t.Stop()
	}
}

func (ep *DiskEndpoint) cleanup() {
	// compute time before which files are expired.
	cutoff := ep.clock.Now().Add(-ep.expiration)
	files, _ := ioutil.ReadDir(ep.path)
	for _, f := range files {
		if isExpired(f.Name(), cutoff) {
			if err := os.Remove(filepath.Join(ep.path, f.Name())); err != nil {
				glog.Warningf("error removing expired disk report: %v", f)
			}
		}
	}
}

func reportName(reportTime time.Time) string {
	return reportPrefix + reportTime.UTC().Format(time.RFC3339) + reportSuffix
}

func isExpired(name string, cutoff time.Time) bool {
	if !strings.HasPrefix(name, reportPrefix) {
		return false
	}
	if !strings.HasSuffix(name, reportSuffix) {
		return false
	}
	t, err := time.Parse(time.RFC3339, name[len(reportPrefix):len(name)-len(reportSuffix)])
	if err != nil {
		return false
	}
	return t.Before(cutoff)
}

func (ep *DiskEndpoint) IsTransient(err error) bool {
	return true
}
