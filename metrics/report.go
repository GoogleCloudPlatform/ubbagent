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

package metrics

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MetricValue holds a single named metric value. Only one of the individual type fields should
// be non-zero.
type MetricValue struct {
	IntValue    int64
	DoubleValue float64
}

func (mv MetricValue) Validate(def Definition) error {
	switch def.Type {
	case IntType:
		if mv.DoubleValue != 0 {
			return fmt.Errorf("double value specified for integer metric: %v", mv.DoubleValue)
		}
		break
	case DoubleType:
		if mv.IntValue != 0 {
			return fmt.Errorf("integer value specified for double metric: %v", mv.IntValue)
		}
		break
	}
	return nil
}

// Report represents an aggregated interval for a unique metric + labels combination.
type MetricReport struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Labels    map[string]string
	Value     MetricValue
}

func (mr MetricReport) Validate(def Definition) error {
	if mr.Name != def.Name {
		return fmt.Errorf("incorrect metric name: %v", mr.Name)
	}
	if mr.StartTime.After(mr.EndTime) {
		return fmt.Errorf("metric %v: StartTime > EndTime: %v > %v", mr.Name, mr.StartTime, mr.EndTime)
	}
	if err := mr.Value.Validate(def); err != nil {
		return fmt.Errorf("metric %v: %v", mr.Name, err)
	}
	return nil
}

// StampedMetricReport is a MetricReport stamped with a unique identifier.
type StampedMetricReport struct {
	MetricReport `json:",inline"`
	Id           string
}

// NewStampedMetricReport creates a new StampedMetricReport with a random, unique identifier.
func NewStampedMetricReport(report MetricReport) StampedMetricReport {
	var stamped StampedMetricReport
	id, err := uuid.NewRandom()
	if err != nil {
		panic(fmt.Sprintf("cannot create uuid for report: %+v", err))
	}
	stamped.Id = id.String()
	stamped.MetricReport = report
	return stamped
}
