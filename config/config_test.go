package config_test

import (
	"reflect"
	"testing"
	"ubbagent/config"
)

func TestParse(t *testing.T) {
	text := `
daemon:
  port: 3456
metrics:
  bufferSeconds: 10
  definitions:
  - name: int-metric
    type: int
  - name: double-metric
    type: double
endpoints:
- name: on_disk
  disk:
    path: /tmp/disk
    expireSeconds: 3600
- name: pubsub
  pubsub:
    keyfile: /tmp/some_key.json
    topic: sometopic
- name: servicecontrol
  servicecontrol:
    keyfile: /tmp/some_key.json
`
	expected := &config.Config{
		Daemon: &config.Daemon{
			Port: 3456,
		},
		Metrics: &config.Metrics{
			BufferSeconds: 10,
			Definitions: []config.MetricDefinition{
				{
					Name: "int-metric",
					Type: "int",
				},
				{
					Name: "double-metric",
					Type: "double",
				},
			},
		},
		Endpoints: []config.Endpoint{
			{
				Name: "on_disk",
				Disk: &config.DiskEndpoint{
					Path:          "/tmp/disk",
					ExpireSeconds: 3600,
				},
			},
			{
				Name: "pubsub",
				PubSub: &config.PubSubEndpoint{
					KeyFile: "/tmp/some_key.json",
					Topic:   "sometopic",
				},
			},
			{
				Name: "servicecontrol",
				ServiceControl: &config.ServiceControlEndpoint{
					KeyFile: "/tmp/some_key.json",
				},
			},
		},
	}

	parsed, err := config.Parse([]byte(text))
	if err != nil {
		t.Fatalf("Error parsing config text: %+v", err)
	}

	if !reflect.DeepEqual(expected, parsed) {
		t.Fatalf("Parsing: expected=%+v; got=%+v", expected, parsed)
	}
}

func TestConfig_Validate(t *testing.T) {

	goodDaemon := &config.Daemon{
		Port: 3333,
	}

	goodMetrics := &config.Metrics{
		BufferSeconds: 10,
		Definitions: []config.MetricDefinition{
			{
				Name: "int-metric",
				Type: "int",
			},
		},
	}

	goodEndpoints := []config.Endpoint{
		{
			Name: "disk",
			Disk: &config.DiskEndpoint{
				Path:          "/tmp/disk",
				ExpireSeconds: 3600,
			},
		},
	}

	t.Run("missing daemon", func(t *testing.T) {
		c := &config.Config{
			Metrics:   goodMetrics,
			Endpoints: goodEndpoints,
		}

		if want, got := "missing daemon section", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("bad port", func(t *testing.T) {
		c := &config.Config{
			Daemon: &config.Daemon{
				Port: -1,
			},
			Metrics:   goodMetrics,
			Endpoints: goodEndpoints,
		}

		if want, got := "daemon: port must be greater than 1024 and less than 65536", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}

		c.Daemon.Port = 65536
		if want, got := "daemon: port must be greater than 1024 and less than 65536", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}

		c.Daemon.Port = 3600
		if err := c.Validate(); err != nil {
			t.Fatalf("Unexpected validation error: +%v", err)
		}
	})

	t.Run("missing metrics", func(t *testing.T) {
		c := &config.Config{
			Daemon:    goodDaemon,
			Endpoints: goodEndpoints,
		}

		if want, got := "missing metrics section", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("invalid metrics", func(t *testing.T) {
		// More tests in the TestMetrics_Validate method.
		c := &config.Config{
			Daemon:    goodDaemon,
			Endpoints: goodEndpoints,
			Metrics: &config.Metrics{
				Definitions: []config.MetricDefinition{
					{Name: "foo", Type: "foo"},
				},
			},
		}
		if want, got := "metric foo: invalid type: foo", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoints", func(t *testing.T) {
		c := &config.Config{
			Daemon:  goodDaemon,
			Metrics: goodMetrics,
		}

		if want, got := "no endpoints defined", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoint name", func(t *testing.T) {
		c := &config.Config{
			Daemon:  goodDaemon,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Disk: &config.DiskEndpoint{
						Path:          "/tmp",
						ExpireSeconds: 10,
					},
				},
			},
		}

		if want, got := "endpoint: missing name", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoint type", func(t *testing.T) {
		c := &config.Config{
			Daemon:  goodDaemon,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
				},
			},
		}

		if want, got := "endpoint foo: missing type configuration", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("too many endpoint types", func(t *testing.T) {
		c := &config.Config{
			Daemon:  goodDaemon,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						Path:          "/tmp",
						ExpireSeconds: 10,
					},
					PubSub: &config.PubSubEndpoint{
						KeyFile: "/tmp/foo",
						Topic:   "bar",
					},
				},
			},
		}

		if want, got := "endpoint foo: multiple type configurations", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("multiple endpoints with the same name", func(t *testing.T) {
		c := &config.Config{
			Daemon:  goodDaemon,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						Path:          "/tmp",
						ExpireSeconds: 10,
					},
				},
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						Path:          "/tmp",
						ExpireSeconds: 10,
					},
				},
			},
		}

		if want, got := "endpoint foo: multiple endpoints with the same name", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("disk endpoint", func(t *testing.T) {
		c := &config.Config{
			Daemon:  goodDaemon,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						Path:          "/tmp",
						ExpireSeconds: 10,
					},
				},
			},
		}

		if want, got := "endpoint foo: multiple type configurations", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})
}

func TestMetrics_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		validConfig := config.Metrics{
			Definitions: []config.MetricDefinition{
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
		duplicateName := config.Metrics{
			Definitions: []config.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric", Type: "int"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := duplicateName.Validate()
		if err == nil || err.Error() != "metric int-metric: duplicate name: int-metric" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})

	t.Run("invalid: invalid type", func(t *testing.T) {
		invalidType := config.Metrics{
			Definitions: []config.MetricDefinition{
				{Name: "int-metric", Type: "int"},
				{Name: "int-metric2", Type: "foo"},
				{Name: "double-metric", Type: "double"},
			},
		}

		err := invalidType.Validate()
		if err == nil || err.Error() != "metric int-metric2: invalid type: foo" {
			t.Fatalf("Expected error, got: %s", err)
		}
	})
}

func TestMetrics_GetMetricDefinition(t *testing.T) {
	validConfig := config.Metrics{
		Definitions: []config.MetricDefinition{
			{Name: "int-metric", Type: "int"},
			{Name: "int-metric2", Type: "int"},
			{Name: "double-metric", Type: "double"},
		},
	}

	expected := config.MetricDefinition{
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
