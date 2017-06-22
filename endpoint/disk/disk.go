package disk

import (
	"encoding/json"
	"github.com/golang/glog"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"ubbagent/clock"
	"ubbagent/endpoint"
	"ubbagent/metrics"
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
}

type diskReport struct {
	name  string
	batch metrics.MetricBatch
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
	r := report.(diskReport)

	jsontext, err := json.Marshal(r.batch)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(ep.path, directoryMode); err != nil {
		return err
	}
	file := path.Join(ep.path, r.name)

	if err := ioutil.WriteFile(file, jsontext, fileMode); err != nil {
		return err
	}
	return nil
}

func (ep *DiskEndpoint) BuildReport(mb metrics.MetricBatch) (endpoint.EndpointReport, error) {
	return diskReport{
		name:  reportName(ep.clock.Now()),
		batch: mb,
	}, nil
}

func (*DiskEndpoint) EmptyReport() endpoint.EndpointReport {
	return diskReport{}
}

// Close instructs the DiskEndpoint's cleanup goroutine to gracefully shutdown. It blocks until the
// operation has completed.
func (ep *DiskEndpoint) Close() error {
	ep.closeOnce.Do(func() {
		ep.quit <- true
	})
	ep.wait.Wait()
	return nil
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
