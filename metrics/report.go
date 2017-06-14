package metrics

import (
	"errors"
	"fmt"
	"time"
	"ubbagent/config"
)

// Report represents a single time-bound collection of metrics.
type MetricReport struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Labels    map[string]string
	Value     MetricValue
}

// MetricValue holds a single named metric value. Only one of the individual type fields should
// be non-zero.
type MetricValue struct {
	IntValue    int64
	DoubleValue float64
}

// MetricBatch is a collection of MetricReports.
type MetricBatch []MetricReport

func (mr *MetricReport) Validate(conf config.Metrics) error {
	def := conf.GetMetricDefinition(mr.Name)
	if def == nil {
		return errors.New(fmt.Sprintf("Unknown metric: %v", mr.Name))
	}
	if mr.StartTime.After(mr.EndTime) {
		return errors.New(fmt.Sprintf("Metric %v: StartTime > EndTime: %v > %v", mr.Name, mr.StartTime, mr.EndTime))
	}
	switch def.Type {
	case config.IntType:
		if mr.Value.DoubleValue != 0 {
			return errors.New(fmt.Sprintf("Metric %v: double value specified for integer metric: %v", mr.Name, mr.Value.DoubleValue))
		}
		break
	case config.DoubleType:
		if mr.Value.IntValue != 0 {
			return errors.New(fmt.Sprintf("Metric %v: integer value specified for double metric: %v", mr.Name, mr.Value.IntValue))
		}
		break
	}
	return nil
}
