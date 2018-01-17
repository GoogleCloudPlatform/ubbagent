// Copyright 2018 Google Inc.
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

type mockInput struct {
	used     bool
	released bool
	report   *metrics.MetricReport
}

func (s *mockInput) AddReport(report metrics.MetricReport) error {
	s.report = &report
	return nil
}

func (s *mockInput) Use() {
	s.used = true
}

func (s *mockInput) Release() error {
	s.released = true
	return nil
}

func TestSelector(t *testing.T) {
	mock1 := &mockInput{}
	mock2 := &mockInput{}

	report1 := metrics.MetricReport{
		Name:      "metric1",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	report2 := metrics.MetricReport{
		Name:      "metric2",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	report3 := metrics.MetricReport{
		Name:      "metric3",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	inputs := map[string]pipeline.Input{
		"metric1": mock1,
		"metric2": mock2,
	}

	s := pipeline.NewSelector(inputs)

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

	t.Run("inputs are used and released", func(t *testing.T) {
		input1 := &mockInput{}
		input2 := &mockInput{}

		s := pipeline.NewSelector(map[string]pipeline.Input{
			"input1": input1,
			"input2": input2,
		})

		if input1.used != true {
			t.Fatalf("expected that input1.used == true")
		}
		if input1.released != false {
			t.Fatalf("expected that input1.released == false")
		}
		if input2.used != true {
			t.Fatalf("expected that input2.used == true")
		}
		if input2.released != false {
			t.Fatalf("expected that input2.released == false")
		}

		s.Release()

		if input1.released != true {
			t.Fatalf("expected that input1.released == true")
		}
		if input2.released != true {
			t.Fatalf("expected that input2.released == true")
		}
	})
}

func TestCompositeInput(t *testing.T) {
	input := &mockInput{}
	add1 := &mockInput{}
	add2 := &mockInput{}

	report := metrics.MetricReport{
		Name:      "metric1",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	composite := pipeline.NewCompositeInput(input, []pipeline.Component{add1, add2})

	if input.used != true {
		t.Fatalf("expected that input.used == true")
	}
	if input.released != false {
		t.Fatalf("expected that input.released == false")
	}
	if add1.used != true {
		t.Fatalf("expected that add1.used == true")
	}
	if add1.released != false {
		t.Fatalf("expected that add1.released == false")
	}
	if add2.used != true {
		t.Fatalf("expected that add2.used == true")
	}
	if add2.released != false {
		t.Fatalf("expected that add2.released == false")
	}

	composite.AddReport(report)

	if !reflect.DeepEqual(input.report, &report) {
		t.Fatalf("expected report to be passed to delegate")
	}

	composite.Release()

	if input.released != true {
		t.Fatalf("expected that input.released == true")
	}
	if add1.released != true {
		t.Fatalf("expected that add1.released == true")
	}
	if add2.released != true {
		t.Fatalf("expected that add2.released == true")
	}
}
