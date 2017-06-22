package config_test

import (
	"encoding/json"
	"github.com/ghodss/yaml"
	"reflect"
	"testing"
	"ubbagent/config"
)

const jsonKeyText = `{
		"type": "service_account",
		"project_id": "bogus",
		"private_key_id": "cfbbc2dba11b1fb1f3cb0f8addca99e9d869300a",
		"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCjuuR6X5QuHsKs\ndUkZr0sMFhvTZAHmaLtpDNxdS/Kpzq5MdFiacZiK6Oj7YIn2fiRhUf6CMtRQrPVA\ngDv61mKP3jhnt1d9xV1WbxUuEGvjmh3HHjfnM8wEQNTm+5vbWfWeeWkmlEiUpxQC\nrHUAmVu/QBmlqDSOX7zWtcvHwLKBvzgqofoA88zrwo33mAyFyZHxgorQGCcSYxCx\nvXM3MhhfBUPGnG/8H8OMXprVlO0uzS7vgjGvnzPr7QlcsR2U2nt/DSu6agVJ7uj2\ncRDqPtC3/8LTB2YFqP4J51KpvdecIQpGSXO+mDGjR245otiuVOL9COS0A+nGui7y\nt2m+pZfPAgMBAAECggEABoSo38YH6D79+A+G/bPZ5srHY8/3N69A/1hIzL5WFE/U\nVCOV6Nwh35r+kcUZ6PkNV5SEAdsUCcVTP6JWt9BhjyGy4eEEd+Frz8Zz6p7pdKjV\n1Mso999xz51ARbmL4PgJ8OBAE1dTfUAaajGxAVS+OIdM1PG0/oAJXue+nefXykyh\nBH2rplFq3QZEl/L4lPOxzUanGMAAusLevTA3OJttKvksgs8Pa3emMKQqEJmXSinS\nPRomj5pVIgQXULbaCKJmyVnHZN7ijJQ1cNjxZ4/pUuu1CN9J+EKpDlyKLUPpkvYi\nn0a4YrH6FCOYiTpJqifBy/Asz9yySD4njLuC4HDkpQKBgQDV4ALaebrLBsOGqatu\n/qF6jDgLXVMgCp9nxYpDgcs4rZEL1kUqOiIOu9SaiZkEipXyNTo5zPyWLa7lyBOq\nsZ6cV8JWHL13iTOmTivldX2HP2VouPE5PKTd1LRs9b4zWAxCBp+JWg/wmugvE1AB\nZyvkXE0Znhu+NBvww440iiFbAwKBgQDD+nmLMbK905aTeXKkpGPZX4qeAgq8dHFL\nt7tM85S3sP/73RFIuDxFF2cFwGToOVpjNHUYGX6Kj8oS0j0Qe4HYb4QBxNaZQeMa\nEo8HXmUYGwd6mX9oxfRjgFzcSe4Uf6dvJq/UNH0FQn2VV/08SINxuTspmxMq1CsA\nUTA2b7KwRQKBgCdfiwbvfAzeXOaQm8feRpoJ8FNfRetTKU9wVWjiHyh7A4XbV3ZT\np2tw9s3QYQQuAzbIx8RWUXXQSS9yKvS0qE999H/n4JV+A60tHPWsMITSjfe+fGIe\nIPfZrbGVeAN5xR/umjYuB1szGWV5N7RaawEqYONDcTYN38ruJWLUvxlDAoGAfsUS\nREj0nzg0SdcgooG4GQ9lYkpd2YPVGa6S2PcjdyNmouxgVtLeIa8+tAi8/T7ESjHP\noLQ1F7plc4FNgNDzsCaKlH5YdrCZD+97V7/m0w4A63xJX2PVb1vENbcY62ebzhmP\nWUxOps1Y4PcW1xzs8e5o58PpRSYTXtQlxMDCLKUCgYAknQmeY7/NKxd6+th9+YF+\nRlExl9xiTxQ++bGwCXEOswgKTOw7LRd+1/CXUDMxy1U3BVDjawoWAtJdVJQRwUp5\nk3otRO1WrgE9VpIU4ahanLHlix11wJK9HHMOXY3u1/4gz8HpbcC6/7xBu1Sv6Omq\n6ZgIQZ3HYpvhmQV8eoxneA==\n-----END PRIVATE KEY-----\n",
		"client_email": "bogus@bogus.iam.bogus.com",
		"client_id": "325892358923048391208",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://accounts.google.com/o/oauth2/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/bogus%40bogus.iam.bogus.com"
  }`

func TestParse(t *testing.T) {
	text := `
identity:
  serviceAccountKey: {
		"type": "service_account",
		"project_id": "bogus",
		"private_key_id": "cfbbc2dba11b1fb1f3cb0f8addca99e9d869300a",
		"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCjuuR6X5QuHsKs\ndUkZr0sMFhvTZAHmaLtpDNxdS/Kpzq5MdFiacZiK6Oj7YIn2fiRhUf6CMtRQrPVA\ngDv61mKP3jhnt1d9xV1WbxUuEGvjmh3HHjfnM8wEQNTm+5vbWfWeeWkmlEiUpxQC\nrHUAmVu/QBmlqDSOX7zWtcvHwLKBvzgqofoA88zrwo33mAyFyZHxgorQGCcSYxCx\nvXM3MhhfBUPGnG/8H8OMXprVlO0uzS7vgjGvnzPr7QlcsR2U2nt/DSu6agVJ7uj2\ncRDqPtC3/8LTB2YFqP4J51KpvdecIQpGSXO+mDGjR245otiuVOL9COS0A+nGui7y\nt2m+pZfPAgMBAAECggEABoSo38YH6D79+A+G/bPZ5srHY8/3N69A/1hIzL5WFE/U\nVCOV6Nwh35r+kcUZ6PkNV5SEAdsUCcVTP6JWt9BhjyGy4eEEd+Frz8Zz6p7pdKjV\n1Mso999xz51ARbmL4PgJ8OBAE1dTfUAaajGxAVS+OIdM1PG0/oAJXue+nefXykyh\nBH2rplFq3QZEl/L4lPOxzUanGMAAusLevTA3OJttKvksgs8Pa3emMKQqEJmXSinS\nPRomj5pVIgQXULbaCKJmyVnHZN7ijJQ1cNjxZ4/pUuu1CN9J+EKpDlyKLUPpkvYi\nn0a4YrH6FCOYiTpJqifBy/Asz9yySD4njLuC4HDkpQKBgQDV4ALaebrLBsOGqatu\n/qF6jDgLXVMgCp9nxYpDgcs4rZEL1kUqOiIOu9SaiZkEipXyNTo5zPyWLa7lyBOq\nsZ6cV8JWHL13iTOmTivldX2HP2VouPE5PKTd1LRs9b4zWAxCBp+JWg/wmugvE1AB\nZyvkXE0Znhu+NBvww440iiFbAwKBgQDD+nmLMbK905aTeXKkpGPZX4qeAgq8dHFL\nt7tM85S3sP/73RFIuDxFF2cFwGToOVpjNHUYGX6Kj8oS0j0Qe4HYb4QBxNaZQeMa\nEo8HXmUYGwd6mX9oxfRjgFzcSe4Uf6dvJq/UNH0FQn2VV/08SINxuTspmxMq1CsA\nUTA2b7KwRQKBgCdfiwbvfAzeXOaQm8feRpoJ8FNfRetTKU9wVWjiHyh7A4XbV3ZT\np2tw9s3QYQQuAzbIx8RWUXXQSS9yKvS0qE999H/n4JV+A60tHPWsMITSjfe+fGIe\nIPfZrbGVeAN5xR/umjYuB1szGWV5N7RaawEqYONDcTYN38ruJWLUvxlDAoGAfsUS\nREj0nzg0SdcgooG4GQ9lYkpd2YPVGa6S2PcjdyNmouxgVtLeIa8+tAi8/T7ESjHP\noLQ1F7plc4FNgNDzsCaKlH5YdrCZD+97V7/m0w4A63xJX2PVb1vENbcY62ebzhmP\nWUxOps1Y4PcW1xzs8e5o58PpRSYTXtQlxMDCLKUCgYAknQmeY7/NKxd6+th9+YF+\nRlExl9xiTxQ++bGwCXEOswgKTOw7LRd+1/CXUDMxy1U3BVDjawoWAtJdVJQRwUp5\nk3otRO1WrgE9VpIU4ahanLHlix11wJK9HHMOXY3u1/4gz8HpbcC6/7xBu1Sv6Omq\n6ZgIQZ3HYpvhmQV8eoxneA==\n-----END PRIVATE KEY-----\n",
		"client_email": "bogus@bogus.iam.bogus.com",
		"client_id": "325892358923048391208",
		"auth_uri": "https://accounts.google.com/o/oauth2/auth",
		"token_uri": "https://accounts.google.com/o/oauth2/token",
		"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
		"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/bogus%40bogus.iam.bogus.com"
  }
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
    reportDir: /tmp/disk
    expireSeconds: 3600
- name: pubsub
  pubsub:
    topic: sometopic
- name: servicecontrol
  servicecontrol:
    serviceName: test-service.bogus.com
    consumerId: project_number:123456
`

	// Run the jsonKeyText variable through ghodss/yaml so that it's formatted the same as the input
	// config.
	key := json.RawMessage{}
	yaml.Unmarshal([]byte(jsonKeyText), &key)

	expected := &config.Config{
		Identity: &config.Identity{
			ServiceAccountKey: key,
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
					ReportDir:     "/tmp/disk",
					ExpireSeconds: 3600,
				},
			},
			{
				Name: "pubsub",
				PubSub: &config.PubSubEndpoint{
					Topic: "sometopic",
				},
			},
			{
				Name: "servicecontrol",
				ServiceControl: &config.ServiceControlEndpoint{
					ServiceName: "test-service.bogus.com",
					ConsumerId:  "project_number:123456",
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
	key := json.RawMessage{}
	yaml.Unmarshal([]byte(jsonKeyText), &key)

	goodIdentity := &config.Identity{
		ServiceAccountKey: key,
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
				ReportDir:     "/tmp/disk",
				ExpireSeconds: 3600,
			},
		},
	}

	t.Run("missing service account key", func(t *testing.T) {
		c := &config.Config{
			Metrics:   goodMetrics,
			Endpoints: goodEndpoints,
			Identity:  &config.Identity{},
		}

		if want, got := "identity: service account key", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing metrics", func(t *testing.T) {
		c := &config.Config{
			Identity:  goodIdentity,
			Endpoints: goodEndpoints,
		}

		if want, got := "missing metrics section", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("invalid metrics", func(t *testing.T) {
		// More tests in the TestMetrics_Validate method.
		c := &config.Config{
			Identity:  goodIdentity,
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
			Identity: goodIdentity,
			Metrics:  goodMetrics,
		}

		if want, got := "no endpoints defined", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoint name", func(t *testing.T) {
		c := &config.Config{
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Disk: &config.DiskEndpoint{
						ReportDir:     "/tmp",
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
			Identity: goodIdentity,
			Metrics:  goodMetrics,
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
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						ReportDir:     "/tmp",
						ExpireSeconds: 10,
					},
					PubSub: &config.PubSubEndpoint{
						Topic: "bar",
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
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						ReportDir:     "/tmp",
						ExpireSeconds: 10,
					},
				},
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						ReportDir:     "/tmp",
						ExpireSeconds: 10,
					},
				},
			},
		}

		if want, got := "endpoint foo: multiple endpoints with the same name", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing service name", func(t *testing.T) {
		c := &config.Config{
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						ConsumerId: "project:foo",
					},
				},
			},
		}

		if want, got := "servicecontrol: missing service name", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing consumer ID", func(t *testing.T) {
		c := &config.Config{
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						ServiceName: "foo.com",
					},
				},
			},
		}

		if want, got := "servicecontrol: missing consumer ID", c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing consumer ID", func(t *testing.T) {
		c := &config.Config{
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						ServiceName: "foo.com",
						ConsumerId:  "bogus",
					},
				},
			},
		}

		if want, got := `servicecontrol: invalid consumer ID (must start with "project:", "projectNumber:", or "apiKey:")`, c.Validate(); got != nil && want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("disk endpoint", func(t *testing.T) {
		c := &config.Config{
			Identity: goodIdentity,
			Metrics:  goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					Disk: &config.DiskEndpoint{
						ReportDir:     "/tmp",
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
