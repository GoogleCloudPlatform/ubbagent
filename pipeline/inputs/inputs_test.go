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

package inputs

import (
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
	"github.com/hashicorp/go-multierror"
)

func TestSelector(t *testing.T) {
	mock1 := testlib.NewMockInput()
	mock2 := testlib.NewMockInput()

	report1 := metrics.MetricReport{
		Name:      "metric1",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			Int64Value: testlib.Int64Ptr(1),
		},
	}

	report2 := metrics.MetricReport{
		Name:      "metric2",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			Int64Value: testlib.Int64Ptr(1),
		},
	}

	report3 := metrics.MetricReport{
		Name:      "metric3",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			Int64Value: testlib.Int64Ptr(1),
		},
	}

	inputs := map[string]pipeline.Input{
		"metric1": mock1,
		"metric2": mock2,
	}

	s := NewSelector(inputs)

	t.Run("proper selection", func(t *testing.T) {
		if err := s.AddReport(report1); err != nil {
			t.Fatalf("unexpected error adding report1: %v", err)
		}
		reports1 := mock1.Reports()
		reports2 := mock2.Reports()
		if len(reports1) != 1 || !reflect.DeepEqual(reports1[0], report1) {
			t.Fatalf("mock1 has unexpected report")
		}
		if len(reports2) != 0 {
			t.Fatalf("mock2 should not have a report")
		}

		if err := s.AddReport(report2); err != nil {
			t.Fatalf("unexpected error adding report2: %v", err)
		}

		reports1 = mock1.Reports()
		reports2 = mock2.Reports()
		if len(reports2) != 1 || !reflect.DeepEqual(reports2[0], report2) {
			t.Fatalf("mock2 has unexpected report")
		}
		if len(reports1) != 0 {
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
		input1 := testlib.NewMockInput()
		input2 := testlib.NewMockInput()

		s := NewSelector(map[string]pipeline.Input{
			"input1": input1,
			"input2": input2,
		})

		if input1.Used != true {
			t.Fatalf("expected that input1.used == true")
		}
		if input1.Released != false {
			t.Fatalf("expected that input1.released == false")
		}
		if input2.Used != true {
			t.Fatalf("expected that input2.used == true")
		}
		if input2.Released != false {
			t.Fatalf("expected that input2.released == false")
		}

		s.Release()

		if input1.Released != true {
			t.Fatalf("expected that input1.released == true")
		}
		if input2.Released != true {
			t.Fatalf("expected that input2.released == true")
		}
	})
}

func TestCallbackInput(t *testing.T) {
	input := testlib.NewMockInput()
	add1 := testlib.NewMockInput()
	add2 := testlib.NewMockInput()

	report := metrics.MetricReport{
		Name:      "metric1",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			Int64Value: testlib.Int64Ptr(1),
		},
	}

	cb := func() error {
		return multierror.Append(add1.Release(), add2.Release()).ErrorOrNil()
	}

	composite := NewCallbackInput(input, cb)

	if input.Used != true {
		t.Fatalf("expected that input.used == true")
	}
	if input.Released != false {
		t.Fatalf("expected that input.released == false")
	}
	if add1.Released != false {
		t.Fatalf("expected that add1.released == false")
	}
	if add2.Released != false {
		t.Fatalf("expected that add2.released == false")
	}

	composite.AddReport(report)

	reports := input.Reports()
	if len(reports) != 1 || !reflect.DeepEqual(reports[0], report) {
		t.Fatalf("expected report to be passed to delegate")
	}

	composite.Release()

	if input.Released != true {
		t.Fatalf("expected that input.released == true")
	}
	if add1.Released != true {
		t.Fatalf("expected that add1.released == true")
	}
	if add2.Released != true {
		t.Fatalf("expected that add2.released == true")
	}
}

func TestLabelingInput(t *testing.T) {

	t.Run("labels are added to report without overwriting", func(t *testing.T) {
		labels := map[string]string{
			"foo1": "bar1",
			"foo2": "bar2",
			"foo3": "bar3",
		}

		report := metrics.MetricReport{
			Name:      "metric1",
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
			Value: metrics.MetricValue{
				Int64Value: testlib.Int64Ptr(1),
			},
			Labels: map[string]string{
				"foo3": "test3",
				"foo5": "test5",
			},
		}

		mockInput := testlib.NewMockInput()
		labelingInput := NewLabelingInput(mockInput, labels)
		if err := labelingInput.AddReport(report); err != nil {
			t.Fatalf("unexpected error adding report: %v", err)
		}
		if !reflect.DeepEqual(mockInput.Reports()[0].Labels, map[string]string{
			"foo1": "bar1",
			"foo2": "bar2",
			"foo3": "test3", // Should not be overwritten
			"foo5": "test5", // Should not be overwritten
		}) {
			t.Fatalf("unexpected labels")
		}
	})

	t.Run("labels added when none exist", func(t *testing.T) {
		labels := map[string]string{
			"foo1": "bar1",
			"foo2": "bar2",
		}

		report := metrics.MetricReport{
			Name:      "metric1",
			StartTime: time.Unix(10, 0),
			EndTime:   time.Unix(11, 0),
			Value: metrics.MetricValue{
				Int64Value: testlib.Int64Ptr(1),
			},
		}

		mockInput := testlib.NewMockInput()
		labelingInput := NewLabelingInput(mockInput, labels)
		if err := labelingInput.AddReport(report); err != nil {
			t.Fatalf("unexpected error adding report: %v", err)
		}
		if !reflect.DeepEqual(mockInput.Reports()[0].Labels, map[string]string{
			"foo1": "bar1",
			"foo2": "bar2",
		}) {
			t.Fatalf("unexpected labels")
		}
	})

	t.Run("delegate is used and released", func(t *testing.T) {
		input := testlib.NewMockInput()
		labelingInput := NewLabelingInput(input, nil)
		if input.Used == true {
			t.Fatalf("expected that input.used == false")
		}
		if input.Released != false {
			t.Fatalf("expected that input.released == false")
		}

		labelingInput.Use()
		if input.Used != true {
			t.Fatalf("expected that input.used == true")
		}
		if input.Released != false {
			t.Fatalf("expected that input.released == false")
		}

		labelingInput.Release()
		if input.Released != true {
			t.Fatalf("expected that input.released == true")
		}
	})
}
