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
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

func TestDiskEndpoint(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_endpoint_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	mc := testlib.NewMockClock()
	mc.SetNow(parseTime("2017-06-19T12:00:00Z"))
	ep := newDiskEndpoint("disk", tmpdir, 10*time.Minute, mc)

	// Make sure we start with an empty dir
	if files, err := ioutil.ReadDir(tmpdir); err != nil {
		t.Fatalf("error listing output directory: %+v", err)
	} else if len(files) != 0 {
		t.Fatalf("output directory contains %v files, expected 0", len(files))
	}

	// Test a single report write
	report1, err := ep.BuildReport(metrics.StampedMetricReport{
		Id: "report1",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric1",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		},
	})
	if err != nil {
		t.Fatalf("error building report: %+v", err)
	}
	if err := ep.Send(report1); err != nil {
		t.Fatalf("error sending report: %+v", err)
	}
	if err := waitForReportCount(tmpdir, 1); err != nil {
		t.Fatalf("error waiting for 1 file in output path: %+v", err)
	}

	mc.SetNow(parseTime("2017-06-19T12:05:00Z"))

	// Test a second report write
	report2, err := ep.BuildReport(metrics.StampedMetricReport{
		Id: "report2",
		MetricReport: metrics.MetricReport{
			Name:      "int-metric1",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: metrics.MetricValue{
				Int64Value: 10,
			},
		},
	})
	if err != nil {
		t.Fatalf("error building report: %+v", err)
	}
	if report2.Id != "report2" {
		t.Fatalf("expected report ID to be 'report2', got: %v", report2.Id)
	}
	if err := ep.Send(report2); err != nil {
		t.Fatalf("error sending report: %+v", err)
	}
	if err := waitForReportCount(tmpdir, 2); err != nil {
		t.Fatalf("error waiting for 2 files in output path: %+v", err)
	}

	// Test that the first report is removed after expiration (10 minutes in this test)
	mc.SetNow(parseTime("2017-06-19T12:11:00Z"))
	if err := waitForReportCount(tmpdir, 1); err != nil {
		t.Fatalf("error waiting for 1 files in output path: %+v", err)
	}

	// Test multiple usages and Release.

	ep.Use()
	ep.Use()

	ep.Release()
	if ep.closed {
		t.Fatal("ep.closed expected to be false")
	}

	ep.Release() // Usage count should be 0; endpoint should be closed.
	if !ep.closed {
		t.Fatal("ep.closed expected to be true")
	}
}

func parseTime(ts string) time.Time {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		panic(err)
	}
	return t
}

func waitForReportCount(dir string, count int) error {
	tmr := time.NewTimer(5 * time.Second)
	tck := time.NewTicker(5 * time.Millisecond)
	defer tmr.Stop()
	defer tck.Stop()
	for {
		select {
		case <-tck.C:
			if files, err := ioutil.ReadDir(dir); err != nil {
				return err
			} else if len(files) == count {
				return nil
			}
		case <-tmr.C:
			return errors.New("timeout")
		}
	}
}
