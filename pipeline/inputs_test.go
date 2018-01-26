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

package pipeline_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/testlib"
)

func TestSelector(t *testing.T) {
	mock1 := testlib.NewMockInput()
	mock2 := testlib.NewMockInput()

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

		s := pipeline.NewSelector(map[string]pipeline.Input{
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

func TestCompositeInput(t *testing.T) {
	input := testlib.NewMockInput()
	add1 := testlib.NewMockInput()
	add2 := testlib.NewMockInput()

	report := metrics.MetricReport{
		Name:      "metric1",
		StartTime: time.Unix(10, 0),
		EndTime:   time.Unix(11, 0),
		Value: metrics.MetricValue{
			IntValue: 1,
		},
	}

	composite := pipeline.NewCompositeInput(input, []pipeline.Component{add1, add2})

	if input.Used != true {
		t.Fatalf("expected that input.used == true")
	}
	if input.Released != false {
		t.Fatalf("expected that input.released == false")
	}
	if add1.Used != true {
		t.Fatalf("expected that add1.used == true")
	}
	if add1.Released != false {
		t.Fatalf("expected that add1.released == false")
	}
	if add2.Used != true {
		t.Fatalf("expected that add2.used == true")
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
