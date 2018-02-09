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
	reportPrefix    = "report"
	reportSuffix    = ".json"
	randomLength    = 5
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

type diskContext struct {
	Name string
}

// NewDiskEndpoint creates a new DiskEndpoint and starts a goroutine that cleans up expired reports
// on disk.
func NewDiskEndpoint(name string, path string, expiration time.Duration) *DiskEndpoint {
	return newDiskEndpoint(name, path, expiration, clock.NewRealClock())
}

func newDiskEndpoint(name string, path string, expiration time.Duration, clock clock.Clock) *DiskEndpoint {
	ep := &DiskEndpoint{
		name:       name,
		path:       path,
		expiration: expiration,
		clock:      clock,
		quit:       make(chan bool, 1),
	}
	ep.wait.Add(1)
	go ep.run(clock.Now())
	return ep
}

func (ep *DiskEndpoint) Name() string {
	return ep.name
}

func (ep *DiskEndpoint) BuildReport(r metrics.StampedMetricReport) (endpoint.EndpointReport, error) {
	return endpoint.NewEndpointReport(r, diskContext{Name: reportName(r, ep.clock.Now())})
}

func (ep *DiskEndpoint) Send(r endpoint.EndpointReport) error {
	dctx := diskContext{}
	err := r.UnmarshalContext(&dctx)
	if err != nil {
		return err
	}
	jsontext, err := json.Marshal(r.StampedMetricReport)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ep.path, directoryMode); err != nil {
		return err
	}
	file := path.Join(ep.path, dctx.Name)

	if err := ioutil.WriteFile(file, jsontext, fileMode); err != nil {
		return err
	}
	return nil
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

func (ep *DiskEndpoint) run(start time.Time) {
	nextFire := start.Add(cleanupInterval)
	for {
		t := ep.clock.NewTimerAt(nextFire)
		select {
		case <-t.GetC():
			ep.cleanup()
		case <-ep.quit:
			ep.wait.Done()
			return
		}
		t.Stop()
		nextFire = nextFire.Add(cleanupInterval)
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

func reportName(report metrics.StampedMetricReport, reportTime time.Time) string {
	var random string
	if len(report.Id) < randomLength {
		random = report.Id
	} else {
		random = report.Id[0:5]
	}
	return reportPrefix + "_" + reportTime.UTC().Format(time.RFC3339) + "_" + random + reportSuffix
}

func isExpired(name string, cutoff time.Time) bool {
	if !strings.HasPrefix(name, reportPrefix) {
		return false
	}
	if !strings.HasSuffix(name, reportSuffix) {
		return false
	}

	parts := strings.Split(name, "_")
	if len(parts) != 3 {
		return false
	}
	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return false
	}
	return t.Before(cutoff)
}

func (ep *DiskEndpoint) IsTransient(err error) bool {
	return true
}
