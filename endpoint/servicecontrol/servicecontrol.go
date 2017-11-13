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
	"context"
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/endpoint"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	servicecontrol "google.golang.org/api/servicecontrol/v1"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
)

const (
	agentIdLabel    = "goog-ubb-agent-id"
	timeout         = 60 * time.Second
)

type ServiceControlEndpoint struct {
	name        string
	serviceName string
	consumerId  string
	agentId     string
	keyData     string
	service     *servicecontrol.Service
	tracker     pipeline.UsageTracker
}

type serviceControlReport struct {
	Id      string
	Request servicecontrol.ReportRequest
}

func (r serviceControlReport) BatchId() string {
	return r.Id
}

// NewServiceControlEndpoint creates a new ServiceControlEndpoint.
func NewServiceControlEndpoint(name, serviceName, agentId string, consumerId string, jsonKey []byte) (*ServiceControlEndpoint, error) {
	config, err := google.JWTConfigFromJSON(jsonKey, servicecontrol.ServicecontrolScope)
	if err != nil {
		return nil, err
	}
	client := config.Client(context.Background())
	client.Timeout = timeout
	service, err := servicecontrol.New(client)
	if err != nil {
		return nil, err
	}
	return newServiceControlEndpoint(name, serviceName, agentId, consumerId, service), nil
}

func newServiceControlEndpoint(name, serviceName, agentId, consumerId string, service *servicecontrol.Service) *ServiceControlEndpoint {
	ep := &ServiceControlEndpoint{
		name:        name,
		serviceName: serviceName,
		agentId:     agentId,
		consumerId:  consumerId,
		service:     service,
	}
	return ep
}

func (ep *ServiceControlEndpoint) Name() string {
	return ep.name
}

func (ep *ServiceControlEndpoint) Send(report endpoint.EndpointReport) error {
	r := report.(*serviceControlReport)
	glog.V(2).Infoln("ServiceControlEndpoint:Send(): serviceName: ", ep.serviceName, " body: ", func() string {
		r_json, _ := r.Request.MarshalJSON()
		return string(r_json)
	}())
	_, err := ep.service.Services.Report(ep.serviceName, &r.Request).Do()
	if err != nil && !googleapi.IsNotModified(err) {
		return err
	}
	glog.V(2).Infoln("ServiceControlEndpoint:Send(): success")
	// TODO(volkman): Handle potential per-operation errors in response body
	return nil
}

func (ep *ServiceControlEndpoint) BuildReport(mb metrics.MetricBatch) (endpoint.EndpointReport, error) {
	ops := make([]*servicecontrol.Operation, len(mb.Reports))
	for i := range mb.Reports {
		m := &mb.Reports[i]
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}

		value := servicecontrol.MetricValue{
			StartTime: m.StartTime.UTC().Format(time.RFC3339Nano),
			EndTime:   m.EndTime.UTC().Format(time.RFC3339Nano),
		}
		if m.Value.IntValue != 0 {
			value.Int64Value = &m.Value.IntValue
		} else if m.Value.DoubleValue != 0 {
			value.DoubleValue = &m.Value.DoubleValue
		}

		ops[i] = &servicecontrol.Operation{
			OperationId: id.String(),
			// ServiceControl requires this field but doesn't indicate what it's supposed to be.
			OperationName: fmt.Sprintf("%v/report", ep.serviceName),
			StartTime:     m.StartTime.UTC().Format(time.RFC3339Nano),
			EndTime:       m.EndTime.UTC().Format(time.RFC3339Nano),
			ConsumerId:    ep.consumerId,
			UserLabels:    m.Labels,
			MetricValueSets: []*servicecontrol.MetricValueSet{
				{
					MetricName:   fmt.Sprintf("%v/%v", ep.serviceName, m.Name),
					MetricValues: []*servicecontrol.MetricValue{&value},
				},
			},
		}

		if ops[i].UserLabels == nil {
			ops[i].UserLabels = make(map[string]string)
		}

		// Add the agent ID label
		ops[i].UserLabels[agentIdLabel] = ep.agentId
	}

	return &serviceControlReport{
		Id: mb.Id,
		Request: servicecontrol.ReportRequest{
			Operations: ops,
		},
	}, nil
}

func (*ServiceControlEndpoint) EmptyReport() endpoint.EndpointReport {
	return &serviceControlReport{}
}

// Use is a no-op. ServiceControlEndpoint doesn't track usage.
func (ep *ServiceControlEndpoint) Use() {}

// Release is a no-op. ServiceControlEndpoint doesn't track usage.
func (ep *ServiceControlEndpoint) Release() error {
	return nil
}

func (ep *ServiceControlEndpoint) IsTransient(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*googleapi.Error)
	if !ok {
		// Some non-http error (perhaps a connection refused or timeout?)
		// We'll retry.
		return true
	}
	// Return true if this is an http error with a 5xx code.
	return ae.Code >= 500 && ae.Code < 600
}
