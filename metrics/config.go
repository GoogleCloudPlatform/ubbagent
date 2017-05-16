package metrics

import (
	"errors"
	"fmt"
)

const (
	IntType    = "int"
	DoubleType = "double"
)

// Config contains the metric definitions that the agent expects to receive. It's intended to be
// specified by the third-party software itself, during a configuration process.
type Config struct {
	// The number of seconds that metrics should be aggregated prior to forwarding
	BufferSeconds int64

	// The list of reportable metrics
	MetricDefinitions []MetricDefinition
}

// MetricDefinition describes a single reportable metric's name and type.
type MetricDefinition struct {
	Name string
	Type string
}

// Validate checks the configuration's validity. Specifically, it must not contain duplicate metric
// definitions, and metric definitions must specify valid type names.
func (c *Config) Validate() error {
	usedNames := make(map[string]bool)
	for _, def := range c.MetricDefinitions {
		if _, exists := usedNames[def.Name]; exists {
			return errors.New(fmt.Sprintf("Duplicate metric name: %s", def.Name))
		}
		usedNames[def.Name] = true

		if def.Type != IntType && def.Type != DoubleType {
			return errors.New(fmt.Sprintf("Metric '%s' has an invalid type: %s", def.Name, def.Type))
		}
	}
	return nil
}

func (c *Config) GetMetricDefinition(name string) *MetricDefinition {
	for _, def := range c.MetricDefinitions {
		if def.Name == name {
			return &def
		}
	}
	return nil
}
