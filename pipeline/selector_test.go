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

package pipeline_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
)

type mockHead struct {
	released bool
	report *metrics.MetricReport
}

func (s *mockHead) AddReport(report metrics.MetricReport) error {
	s.report = &report
	return nil
}

func (s *mockHead) Use() {}

func (s *mockHead) Release() error {
	s.released = true
	return nil
}

func TestSelector(t *testing.T) {
	mock1 := &mockHead{}
	mock2 := &mockHead{}

	report1 := metrics.MetricReport{
		Name: "metric1",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	report2 := metrics.MetricReport{
		Name: "metric2",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	report3 := metrics.MetricReport{
		Name: "metric3",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	heads := map[string]pipeline.Head{
		"metric1": mock1,
		"metric2": mock2,
	}

	s := pipeline.NewSelector(heads)

	t.Run("proper selection", func(t *testing.T) {
		if err := s.AddReport(report1); err != nil {
			t.Fatalf("unexpected error adding report1: %v", err)
		}
		if mock1.report == nil || !reflect.DeepEqual(mock1.report, &report1) {
			t.Fatalf("mock1 has unexpected report")
		}
		if mock2.report != nil {
			t.Fatalf("mock2 should not have a report")
		}

		mock1.report = nil
		mock2.report = nil

		if err := s.AddReport(report2); err != nil {
			t.Fatalf("unexpected error adding report2: %v", err)
		}
		if mock2.report == nil || !reflect.DeepEqual(mock2.report, &report2) {
			t.Fatalf("mock2 has unexpected report")
		}
		if mock1.report != nil {
			t.Fatalf("mock1 should not have a report")
		}
	})

	t.Run("invalid name", func(t *testing.T) {
		err := s.AddReport(report3)
		if err == nil {
			t.Fatalf("expected error when adding report3, got none")
		}
		if err.Error() != "selector: unknown metric: metric3" {
			t.Fatalf("unexpected error message: %v", err.Error())
		}
	})
}
