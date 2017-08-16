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

package servicecontrol

import (
	"encoding/json"
	"google.golang.org/api/servicecontrol/v1"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
)

type recordingHandler struct {
	req  *http.Request
	body []byte
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.req = r
	var err error
	h.body, err = ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	resp := &servicecontrol.ReportResponse{}
	respJson, err := resp.MarshalJSON()
	if err != nil {
		panic(err)
	}
	w.Write(respJson)
}

func TestServiceControlEndpoint(t *testing.T) {
	handler := &recordingHandler{}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	svc, err := servicecontrol.New(http.DefaultClient)
	if err != nil {
		t.Fatalf("Error creating client: %+v", err)
	}

	// Point the service's path at our mock HTTP instance.
	svc.BasePath = ts.URL

	ep := newServiceControlEndpoint("servicecontrol", "test-service.appspot.com", "unique-agent-id", "project_number:1234567", svc)

	t.Run("Report idempotence", func(t *testing.T) {
		// Test a single report write
		report1, err := ep.BuildReport(metrics.MetricBatch{
			metrics.MetricReport{
				Name:      "int-metric1",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					IntValue: 10,
				},
			},
		})
		if err != nil {
			t.Fatalf("error building report: %+v", err)
		}
		if err := ep.Send(report1); err != nil {
			t.Fatalf("error sending report: %+v", err)
		}

		// Test that a second send of the same report sends the same body
		body1, _ := ioutil.ReadAll(handler.req.Body)
		if err := ep.Send(report1); err != nil {
			t.Fatalf("error sending report: %+v", err)
		}
		body2, _ := ioutil.ReadAll(handler.req.Body)
		if !reflect.DeepEqual(body1, body2) {
			t.Fatal("two requests from same report were not equal")
		}
	})

	t.Run("Sent contents are correct", func(t *testing.T) {
		// Test a single report write
		report1, err := ep.BuildReport(metrics.MetricBatch{
			metrics.MetricReport{
				Name:      "int-metric",
				StartTime: time.Unix(0, 0),
				EndTime:   time.Unix(1, 0),
				Value: metrics.MetricValue{
					IntValue: 10,
				},
				BillingName: "com.googleapis/services/test-service/IntMetric",
			},
			metrics.MetricReport{
				Name:      "double-metric",
				StartTime: time.Unix(2, 0),
				EndTime:   time.Unix(3, 0),
				Value: metrics.MetricValue{
					DoubleValue: 20,
				},
				Labels: map[string]string{
					"foo": "bar",
				},
				BillingName: "com.googleapis/services/test-service/DoubleMetric",
			},
		})
		if err != nil {
			t.Fatalf("error building report: %+v", err)
		}
		if err := ep.Send(report1); err != nil {
			t.Fatalf("error sending report: %+v", err)
		}

		var intVal int64 = 10
		var doubleVal float64 = 20

		expectedOps := []*servicecontrol.Operation{
			{
				OperationName: "test-service.appspot.com/report",
				StartTime:     time.Unix(0, 0).UTC().Format(time.RFC3339Nano),
				EndTime:       time.Unix(1, 0).UTC().Format(time.RFC3339Nano),
				ConsumerId:    "project_number:1234567",
				UserLabels: map[string]string{
					"goog-ubb-agent-id": "unique-agent-id",
				},
				MetricValueSets: []*servicecontrol.MetricValueSet{
					{
						MetricName: "test-service.appspot.com/int-metric",
						MetricValues: []*servicecontrol.MetricValue{
							{
								StartTime:  time.Unix(0, 0).UTC().Format(time.RFC3339Nano),
								EndTime:    time.Unix(1, 0).UTC().Format(time.RFC3339Nano),
								Int64Value: &intVal,
								Labels: map[string]string{
									"cloudbilling.googleapis.com/argentum_metric_id": "com.googleapis/services/test-service/IntMetric",
								},
							},
						},
					},
				},
			},
			{
				OperationName: "test-service.appspot.com/report",
				StartTime:     time.Unix(2, 0).UTC().Format(time.RFC3339Nano),
				EndTime:       time.Unix(3, 0).UTC().Format(time.RFC3339Nano),
				ConsumerId:    "project_number:1234567",
				UserLabels: map[string]string{
					"goog-ubb-agent-id": "unique-agent-id",
					"foo":               "bar",
				},
				MetricValueSets: []*servicecontrol.MetricValueSet{
					{
						MetricName: "test-service.appspot.com/double-metric",
						MetricValues: []*servicecontrol.MetricValue{
							{
								StartTime:   time.Unix(2, 0).UTC().Format(time.RFC3339Nano),
								EndTime:     time.Unix(3, 0).UTC().Format(time.RFC3339Nano),
								DoubleValue: &doubleVal,
								Labels: map[string]string{
									"cloudbilling.googleapis.com/argentum_metric_id": "com.googleapis/services/test-service/DoubleMetric",
								},
							},
						},
					},
				},
			},
		}

		req := servicecontrol.ReportRequest{}
		if err := json.Unmarshal(handler.body, &req); err != nil {
			t.Fatalf("unmarshalling request: %+v", err)
		}

		// First we check to make sure each Operation has a unique ID, then zero the IDs
		// prior to comparing the rest of the values.
		usedIds := make(map[string]bool)
		for _, op := range req.Operations {
			if op.OperationId == "" {
				t.Fatal("found empty OperationId")
			}
			if usedIds[op.OperationId] {
				t.Fatalf("found reused OperationId: %v", op.OperationId)
			}
			usedIds[op.OperationId] = true
			op.OperationId = ""
		}

		if !reflect.DeepEqual(req.Operations, expectedOps) {
			t.Fatal("request operations didn't match expected")
		}
	})

	// Test that close returns successfully.
	ep.Close()
}
