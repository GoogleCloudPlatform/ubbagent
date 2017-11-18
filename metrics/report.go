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

	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/google/uuid"
)

// Report represents an aggregated interval for a unique metric + labels combination.
type MetricReport struct {
	Name        string
	StartTime   time.Time
	EndTime     time.Time
	Labels      map[string]string
	Value       MetricValue
}

// MetricValue holds a single named metric value. Only one of the individual type fields should
// be non-zero.
type MetricValue struct {
	IntValue    int64
	DoubleValue float64
}

// MetricBatch is a collection of MetricReports with an identifier.
type MetricBatch struct {
	Id string
	Reports []MetricReport
}

// NewMetricBatch creates a new MetricBatch with a random, unique identifier.
func NewMetricBatch(reports []MetricReport) (MetricBatch, error) {
	var batch MetricBatch
	id, err := uuid.NewRandom()
	if err != nil {
		return batch, err
	}
	batch.Id = id.String()
	batch.Reports = reports
	return batch, nil
}

func (mr MetricReport) Validate(def config.MetricDefinition) error {
	if mr.Name != def.Name {
		return fmt.Errorf("incorrect metric name: %v", mr.Name)
	}
	if mr.StartTime.After(mr.EndTime) {
		return fmt.Errorf("metric %v: StartTime > EndTime: %v > %v", mr.Name, mr.StartTime, mr.EndTime)
	}
	switch def.Type {
	case config.IntType:
		if mr.Value.DoubleValue != 0 {
			return fmt.Errorf("metric %v: double value specified for integer metric: %v", mr.Name, mr.Value.DoubleValue)
		}
		break
	case config.DoubleType:
		if mr.Value.IntValue != 0 {
			return fmt.Errorf("metric %v: integer value specified for double metric: %v", mr.Name, mr.Value.IntValue)
		}
		break
	}
	return nil
}
