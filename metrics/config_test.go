package metrics_test

import (
	"testing"
	"ubbagent/metrics"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		validConfig := metrics.Config{
			MetricDefinitions: []metrics.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric2", Type: "int"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := validConfig.Validate()
		if err != nil {
			t.Fatalf("Expected no error, got: %s", err)
		}
	})

	t.Run("invalid: duplicate metric", func(t *testing.T) {
		duplicateName := metrics.Config{
			MetricDefinitions: []metrics.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric", Type: "int"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := duplicateName.Validate()
		if err == nil || err.Error() != "Duplicate metric name: int-metric" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: invalid type", func(t *testing.T) {
		invalidType := metrics.Config{
			MetricDefinitions: []metrics.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric2", Type: "foo"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := invalidType.Validate()
		if err == nil || err.Error() != "Metric 'int-metric2' has an invalid type: foo" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})
}

func TestConfig_GetMetricDefinition(t *testing.T) {
	validConfig := metrics.Config{
		MetricDefinitions: []metrics.MetricDefinition{
			{Name: "int-metric", Type: "int"},
			{Name: "int-metric2", Type: "int"},
			{Name: "double-metric", Type: "double"},
		},
	}

	expected := metrics.MetricDefinition{
		Name: "int-metric2",
		Type: "int",
	}
	actual := validConfig.GetMetricDefinition("int-metric2")
	if *actual != expected {
		t.Fatalf("Expected: %s, got: %s", expected, actual)
	}

	actual = validConfig.GetMetricDefinition("bogus")
	if actual != nil {
		t.Fatalf("Expected: nil, got: %s", actual)
	}
}
