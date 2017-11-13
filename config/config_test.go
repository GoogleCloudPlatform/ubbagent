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

package config_test

import (
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/ubbagent/config"
	"github.com/ghodss/yaml"
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
identities:
  - name: gcp
    gcp:
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
    identity: gcp
    serviceName: test-service.bogus.com
    consumerId: project_number:123456
`

	// Run the jsonKeyText variable through ghodss/yaml so that it's formatted the same as the input
	// config.
	key := config.LiteralServiceAccountKey{}
	yaml.Unmarshal([]byte(jsonKeyText), &key)

	expected := &config.Config{
		Identities: []config.Identity{
			{
				Name: "gcp",
				GCP: &config.GCPIdentity{
					ServiceAccountKey: key,
				},
			},
		},
		Metrics: &config.Metrics{
			BufferSeconds: 10,
			Definitions: []config.MetricDefinition{
				{
					Name:        "int-metric",
					Type:        "int",
				},
				{
					Name:        "double-metric",
					Type:        "double",
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

func TestParse_EncodedKey(t *testing.T) {
	text := `
identities:
  - name: gcp
    gcp:
      encodedServiceAccountKey: ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAiYm9ndXMiLAogICJwcml2YXRlX2tleV9pZCI6ICJjZmJiYzJkYmExMWIxZmIxZjNjYjBmOGFkZGNhOTllOWQ4NjkzMDBhIiwKICAicHJpdmF0ZV9rZXkiOiAiLS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tXG5NSUlFdkFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLWXdnZ1NpQWdFQUFvSUJBUUNqdXVSNlg1UXVIc0tzXG5kVWtacjBzTUZodlRaQUhtYUx0cEROeGRTL0twenE1TWRGaWFjWmlLNk9qN1lJbjJmaVJoVWY2Q010UlFyUFZBXG5nRHY2MW1LUDNqaG50MWQ5eFYxV2J4VXVFR3ZqbWgzSEhqZm5NOHdFUU5UbSs1dmJXZldlZVdrbWxFaVVweFFDXG5ySFVBbVZ1L1FCbWxxRFNPWDd6V3Rjdkh3TEtCdnpncW9mb0E4OHpyd28zM21BeUZ5Wkh4Z29yUUdDY1NZeEN4XG52WE0zTWhoZkJVUEduRy84SDhPTVhwclZsTzB1elM3dmdqR3ZuelByN1FsY3NSMlUybnQvRFN1NmFnVko3dWoyXG5jUkRxUHRDMy84TFRCMllGcVA0SjUxS3B2ZGVjSVFwR1NYTyttREdqUjI0NW90aXVWT0w5Q09TMEErbkd1aTd5XG50Mm0rcFpmUEFnTUJBQUVDZ2dFQUJvU28zOFlINkQ3OStBK0cvYlBaNXNySFk4LzNONjlBLzFoSXpMNVdGRS9VXG5WQ09WNk53aDM1citrY1VaNlBrTlY1U0VBZHNVQ2NWVFA2Sld0OUJoanlHeTRlRUVkK0ZyejhaejZwN3BkS2pWXG4xTXNvOTk5eHo1MUFSYm1MNFBnSjhPQkFFMWRUZlVBYWFqR3hBVlMrT0lkTTFQRzAvb0FKWHVlK25lZlh5a3loXG5CSDJycGxGcTNRWkVsL0w0bFBPeHpVYW5HTUFBdXNMZXZUQTNPSnR0S3Zrc2dzOFBhM2VtTUtRcUVKbVhTaW5TXG5QUm9tajVwVklnUVhVTGJhQ0tKbXlWbkhaTjdpakpRMWNOanhaNC9wVXV1MUNOOUorRUtwRGx5S0xVUHBrdllpXG5uMGE0WXJINkZDT1lpVHBKcWlmQnkvQXN6OXl5U0Q0bmpMdUM0SERrcFFLQmdRRFY0QUxhZWJyTEJzT0dxYXR1XG4vcUY2akRnTFhWTWdDcDlueFlwRGdjczRyWkVMMWtVcU9pSU91OVNhaVprRWlwWHlOVG81elB5V0xhN2x5Qk9xXG5zWjZjVjhKV0hMMTNpVE9tVGl2bGRYMkhQMlZvdVBFNVBLVGQxTFJzOWI0eldBeENCcCtKV2cvd211Z3ZFMUFCXG5aeXZrWEUwWm5odStOQnZ3dzQ0MGlpRmJBd0tCZ1FERCtubUxNYks5MDVhVGVYS2twR1BaWDRxZUFncThkSEZMXG50N3RNODVTM3NQLzczUkZJdUR4RkYyY0Z3R1RvT1Zwak5IVVlHWDZLajhvUzBqMFFlNEhZYjRRQnhOYVpRZU1hXG5FbzhIWG1VWUd3ZDZtWDlveGZSamdGemNTZTRVZjZkdkpxL1VOSDBGUW4yVlYvMDhTSU54dVRzcG14TXExQ3NBXG5VVEEyYjdLd1JRS0JnQ2RmaXdidmZBemVYT2FRbThmZVJwb0o4Rk5mUmV0VEtVOXdWV2ppSHloN0E0WGJWM1pUXG5wMnR3OXMzUVlRUXVBemJJeDhSV1VYWFFTUzl5S3ZTMHFFOTk5SC9uNEpWK0E2MHRIUFdzTUlUU2pmZStmR0llXG5JUGZacmJHVmVBTjV4Ui91bWpZdUIxc3pHV1Y1TjdSYWF3RXFZT05EY1RZTjM4cnVKV0xVdnhsREFvR0Fmc1VTXG5SRWowbnpnMFNkY2dvb0c0R1E5bFlrcGQyWVBWR2E2UzJQY2pkeU5tb3V4Z1Z0TGVJYTgrdEFpOC9UN0VTakhQXG5vTFExRjdwbGM0Rk5nTkR6c0NhS2xINVlkckNaRCs5N1Y3L20wdzRBNjN4SlgyUFZiMXZFTmJjWTYyZWJ6aG1QXG5XVXhPcHMxWTRQY1cxeHpzOGU1bzU4UHBSU1lUWHRRbHhNRENMS1VDZ1lBa25RbWVZNy9OS3hkNit0aDkrWUYrXG5SbEV4bDl4aVR4USsrYkd3Q1hFT3N3Z0tUT3c3TFJkKzEvQ1hVRE14eTFVM0JWRGphd29XQXRKZFZKUVJ3VXA1XG5rM290Uk8xV3JnRTlWcElVNGFoYW5MSGxpeDExd0pLOUhITU9YWTN1MS80Z3o4SHBiY0M2Lzd4QnUxU3Y2T21xXG42WmdJUVozSFlwdmhtUVY4ZW94bmVBPT1cbi0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS1cbiIsCiAgImNsaWVudF9lbWFpbCI6ICJib2d1c0Bib2d1cy5pYW0uYm9ndXMuY29tIiwKICAiY2xpZW50X2lkIjogIjMyNTg5MjM1ODkyMzA0ODM5MTIwOCIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L2JvZ3VzJTQwYm9ndXMuaWFtLmJvZ3VzLmNvbSIKfQo=
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
    identity: gcp
    serviceName: test-service.bogus.com
    consumerId: project_number:123456
`

	expectedKey := config.LiteralServiceAccountKey{}
	yaml.Unmarshal([]byte(jsonKeyText), &expectedKey)

	parsed, err := config.Parse([]byte(text))
	if err != nil {
		t.Fatalf("Error parsing config text: %+v", err)
	}

	if want, got := expectedKey, parsed.Identities[0].GCP.EncodedServiceAccountKey; !yamlEqual(want, got) {
		t.Fatalf("Parsing encoded key: expected=%+v; got=%+v", string(want), string(got))
	}
}

func TestConfig_Validate(t *testing.T) {
	literalGcpKey := config.LiteralServiceAccountKey{}
	yaml.Unmarshal([]byte(jsonKeyText), &literalGcpKey)

	goodIdentities := []config.Identity{
		{
			Name: "gcp",
			GCP: &config.GCPIdentity{
				ServiceAccountKey: literalGcpKey,
			},
		},
	}

	goodMetrics := &config.Metrics{
		BufferSeconds: 10,
		Definitions: []config.MetricDefinition{
			{
				Name:        "int-metric",
				Type:        "int",
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
			Metrics: goodMetrics,
			Endpoints: goodEndpoints,
			Identities: []config.Identity{
				{
					Name: "gcp",
					GCP: &config.GCPIdentity{},
				},
			},
		}

		if want, got := "identity: missing service account key", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("too many service account keys", func(t *testing.T) {

		encodedKeyText := "\"ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAiYm9ndXMiLAogICJwcml2YXRlX2tleV9pZCI6ICJjZmJiYzJkYmExMWIxZmIxZjNjYjBmOGFkZGNhOTllOWQ4NjkzMDBhIiwKICAicHJpdmF0ZV9rZXkiOiAiLS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tXG5NSUlFdkFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLWXdnZ1NpQWdFQUFvSUJBUUNqdXVSNlg1UXVIc0tzXG5kVWtacjBzTUZodlRaQUhtYUx0cEROeGRTL0twenE1TWRGaWFjWmlLNk9qN1lJbjJmaVJoVWY2Q010UlFyUFZBXG5nRHY2MW1LUDNqaG50MWQ5eFYxV2J4VXVFR3ZqbWgzSEhqZm5NOHdFUU5UbSs1dmJXZldlZVdrbWxFaVVweFFDXG5ySFVBbVZ1L1FCbWxxRFNPWDd6V3Rjdkh3TEtCdnpncW9mb0E4OHpyd28zM21BeUZ5Wkh4Z29yUUdDY1NZeEN4XG52WE0zTWhoZkJVUEduRy84SDhPTVhwclZsTzB1elM3dmdqR3ZuelByN1FsY3NSMlUybnQvRFN1NmFnVko3dWoyXG5jUkRxUHRDMy84TFRCMllGcVA0SjUxS3B2ZGVjSVFwR1NYTyttREdqUjI0NW90aXVWT0w5Q09TMEErbkd1aTd5XG50Mm0rcFpmUEFnTUJBQUVDZ2dFQUJvU28zOFlINkQ3OStBK0cvYlBaNXNySFk4LzNONjlBLzFoSXpMNVdGRS9VXG5WQ09WNk53aDM1citrY1VaNlBrTlY1U0VBZHNVQ2NWVFA2Sld0OUJoanlHeTRlRUVkK0ZyejhaejZwN3BkS2pWXG4xTXNvOTk5eHo1MUFSYm1MNFBnSjhPQkFFMWRUZlVBYWFqR3hBVlMrT0lkTTFQRzAvb0FKWHVlK25lZlh5a3loXG5CSDJycGxGcTNRWkVsL0w0bFBPeHpVYW5HTUFBdXNMZXZUQTNPSnR0S3Zrc2dzOFBhM2VtTUtRcUVKbVhTaW5TXG5QUm9tajVwVklnUVhVTGJhQ0tKbXlWbkhaTjdpakpRMWNOanhaNC9wVXV1MUNOOUorRUtwRGx5S0xVUHBrdllpXG5uMGE0WXJINkZDT1lpVHBKcWlmQnkvQXN6OXl5U0Q0bmpMdUM0SERrcFFLQmdRRFY0QUxhZWJyTEJzT0dxYXR1XG4vcUY2akRnTFhWTWdDcDlueFlwRGdjczRyWkVMMWtVcU9pSU91OVNhaVprRWlwWHlOVG81elB5V0xhN2x5Qk9xXG5zWjZjVjhKV0hMMTNpVE9tVGl2bGRYMkhQMlZvdVBFNVBLVGQxTFJzOWI0eldBeENCcCtKV2cvd211Z3ZFMUFCXG5aeXZrWEUwWm5odStOQnZ3dzQ0MGlpRmJBd0tCZ1FERCtubUxNYks5MDVhVGVYS2twR1BaWDRxZUFncThkSEZMXG50N3RNODVTM3NQLzczUkZJdUR4RkYyY0Z3R1RvT1Zwak5IVVlHWDZLajhvUzBqMFFlNEhZYjRRQnhOYVpRZU1hXG5FbzhIWG1VWUd3ZDZtWDlveGZSamdGemNTZTRVZjZkdkpxL1VOSDBGUW4yVlYvMDhTSU54dVRzcG14TXExQ3NBXG5VVEEyYjdLd1JRS0JnQ2RmaXdidmZBemVYT2FRbThmZVJwb0o4Rk5mUmV0VEtVOXdWV2ppSHloN0E0WGJWM1pUXG5wMnR3OXMzUVlRUXVBemJJeDhSV1VYWFFTUzl5S3ZTMHFFOTk5SC9uNEpWK0E2MHRIUFdzTUlUU2pmZStmR0llXG5JUGZacmJHVmVBTjV4Ui91bWpZdUIxc3pHV1Y1TjdSYWF3RXFZT05EY1RZTjM4cnVKV0xVdnhsREFvR0Fmc1VTXG5SRWowbnpnMFNkY2dvb0c0R1E5bFlrcGQyWVBWR2E2UzJQY2pkeU5tb3V4Z1Z0TGVJYTgrdEFpOC9UN0VTakhQXG5vTFExRjdwbGM0Rk5nTkR6c0NhS2xINVlkckNaRCs5N1Y3L20wdzRBNjN4SlgyUFZiMXZFTmJjWTYyZWJ6aG1QXG5XVXhPcHMxWTRQY1cxeHpzOGU1bzU4UHBSU1lUWHRRbHhNRENMS1VDZ1lBa25RbWVZNy9OS3hkNit0aDkrWUYrXG5SbEV4bDl4aVR4USsrYkd3Q1hFT3N3Z0tUT3c3TFJkKzEvQ1hVRE14eTFVM0JWRGphd29XQXRKZFZKUVJ3VXA1XG5rM290Uk8xV3JnRTlWcElVNGFoYW5MSGxpeDExd0pLOUhITU9YWTN1MS80Z3o4SHBiY0M2Lzd4QnUxU3Y2T21xXG42WmdJUVozSFlwdmhtUVY4ZW94bmVBPT1cbi0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS1cbiIsCiAgImNsaWVudF9lbWFpbCI6ICJib2d1c0Bib2d1cy5pYW0uYm9ndXMuY29tIiwKICAiY2xpZW50X2lkIjogIjMyNTg5MjM1ODkyMzA0ODM5MTIwOCIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L2JvZ3VzJTQwYm9ndXMuaWFtLmJvZ3VzLmNvbSIKfQo=\""
		encodedGcpKey := config.EncodedServiceAccountKey{}
		yaml.Unmarshal([]byte(encodedKeyText), &encodedGcpKey)

		c := &config.Config{
			Metrics:   goodMetrics,
			Endpoints: goodEndpoints,
			Identities: []config.Identity{
				{
					Name: "gcp",
					GCP: &config.GCPIdentity{
						ServiceAccountKey:        literalGcpKey,
						EncodedServiceAccountKey: encodedGcpKey,
					},
				},
			},
		}

		if want, got := "identity: too many service account keys", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing metrics", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Endpoints:  goodEndpoints,
		}

		if want, got := "missing metrics section", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("invalid metrics", func(t *testing.T) {
		// More tests in the TestMetrics_Validate method.
		c := &config.Config{
			Identities: goodIdentities,
			Endpoints: goodEndpoints,
			Metrics: &config.Metrics{
				Definitions: []config.MetricDefinition{
					{Name: "foo", Type: "foo"},
				},
			},
		}
		if want, got := "metric foo: invalid type: foo", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoints", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics:    goodMetrics,
		}

		if want, got := "no endpoints defined", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoint name", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Disk: &config.DiskEndpoint{
						ReportDir:     "/tmp",
						ExpireSeconds: 10,
					},
				},
			},
		}

		if want, got := "endpoint: missing name", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing endpoint type", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
				},
			},
		}

		if want, got := "endpoint foo: missing type configuration", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("too many endpoint types", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
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

		if want, got := "endpoint foo: multiple type configurations", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("multiple endpoints with the same name", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
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

		if want, got := "endpoint foo: multiple endpoints with the same name", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing identity name", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						ServiceName: "foo.com",
						ConsumerId:  "project:foo",
					},
				},
			},
		}

		if want, got := "servicecontrol: missing identity name", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("nonexistent identity name", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						Identity:    "foo",
						ServiceName: "foo.com",
						ConsumerId:  "project:foo",
					},
				},
			},
		}

		if want, got := "servicecontrol: nonexistent identity: foo", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing service name", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						Identity:   "gcp",
						ConsumerId: "project:foo",
					},
				},
			},
		}

		if want, got := "servicecontrol: missing service name", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("missing consumer ID", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						Identity:    "gcp",
						ServiceName: "foo.com",
					},
				},
			},
		}

		if want, got := "servicecontrol: missing consumer ID", c.Validate(); got == nil || want != got.Error() {
			t.Fatalf("wanted: %+v, got: %+v", want, got)
		}
	})

	t.Run("invalid consumer ID", func(t *testing.T) {
		c := &config.Config{
			Identities: goodIdentities,
			Metrics: goodMetrics,
			Endpoints: []config.Endpoint{
				{
					Name: "foo",
					ServiceControl: &config.ServiceControlEndpoint{
						Identity:    "gcp",
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
}

func yamlEqual(want, got []byte) bool {
	w := make(map[string]interface{})
	g := make(map[string]interface{})

	if err := yaml.Unmarshal(want, &w); err != nil {
		return false
	}
	if err := yaml.Unmarshal(got, &g); err != nil {
		return false
	}

	return reflect.DeepEqual(w, g)
}
