package disk

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"
	"ubbagent/clock"
	"ubbagent/metrics"
)

func TestDiskEndpoint(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "disk_endpoint_test")
	if err != nil {
		t.Fatalf("Unable to create temp directory: %+v", err)
	}
	defer os.RemoveAll(tmpdir)

	mc := clock.NewMockClock()
	mc.SetNow(parseTime("2017-06-19T12:00:00Z"))
	ep := newDiskEndpoint("disk", tmpdir, 10*time.Minute, mc)

	// Make sure we start with an empty dir
	if files, err := ioutil.ReadDir(tmpdir); err != nil {
		t.Fatalf("error listing output directory: %+v", err)
	} else if len(files) != 0 {
		t.Fatalf("output directory contains %v files, expected 0", len(files))
	}

	// Test a single report write
	report1, err := ep.BuildReport(metrics.MetricBatch{
		metrics.MetricReport{
			Name:      "int-metric1",
			StartTime: time.Unix(0, 0),
			EndTime:   time.Unix(1, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
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
	report2, err := ep.BuildReport(metrics.MetricBatch{
		metrics.MetricReport{
			Name:      "int-metric1",
			StartTime: time.Unix(2, 0),
			EndTime:   time.Unix(3, 0),
			Value: metrics.MetricValue{
				IntValue: 10,
			},
		},
	})
	if err != nil {
		t.Fatalf("error building report: %+v", err)
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

	// Test that close returns successfully.
	ep.Close()
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
