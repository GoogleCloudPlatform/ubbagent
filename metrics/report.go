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
	"reflect"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/util"
	"github.com/google/uuid"
)

// MetricValue holds a single named metric value. Only one of the individual type fields should
// be non-nil.
type MetricValue struct {
	Int64Value  *int64   `json:"int64Value,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}

// Validate returns an error if the metric value does not match its definition.
func (mv MetricValue) Validate(def Definition) error {
	switch def.Type {
	case IntType:
		if mv.DoubleValue != nil {
			return fmt.Errorf("double value specified for integer metric: %v", *mv.DoubleValue)
		}
		break
	case DoubleType:
		if mv.Int64Value != nil {
			return fmt.Errorf("integer value specified for double metric: %v", *mv.Int64Value)
		}
		break
	}

	return nil
}

// MetricReport represents an aggregated interval for a unique metric + labels combination.
type MetricReport struct {
	Name      string            `json:"name"`
	StartTime time.Time         `json:"startTime"`
	EndTime   time.Time         `json:"endTime"`
	Labels    map[string]string `json:"labels"`
	Value     MetricValue       `json:"value"`
}

// Equal returns if the two MetricReports are the same.
func (mr MetricReport) Equal(other MetricReport) bool {
	// Time object equality must be checked using `Time.Equal`,
	// not `==` or `reflect.DeepEqual`.
	// See https://github.com/golang/go/issues/17875
	return mr.Name == other.Name &&
		mr.StartTime.Equal(other.StartTime) &&
		mr.EndTime.Equal(other.EndTime) &&
		reflect.DeepEqual(mr.Labels, other.Labels) &&
		reflect.DeepEqual(mr.Value, other.Value)
}

// Copy returns a deep copy of the MetricReport
func (mr MetricReport) Copy() MetricReport {
	dup := mr
	if mr.Value.Int64Value != nil {
		dup.Value.Int64Value = util.NewInt64(*mr.Value.Int64Value)
	}
	if mr.Value.DoubleValue != nil {
		dup.Value.DoubleValue = util.NewFloat64(*mr.Value.DoubleValue)
	}
	return dup
}

// Validate returns an error if the report does not match its definition.
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

// Equal returns if the two StampedMetricReports are the same.
func (smr StampedMetricReport) Equal(other StampedMetricReport) bool {
	return smr.Id == other.Id &&
		smr.MetricReport.Equal(other.MetricReport)
}
