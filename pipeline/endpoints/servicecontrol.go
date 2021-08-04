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

package endpoints

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/metrics"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/golang/glog"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/servicecontrol/v1"
)

const (
	agentIdLabel      = "goog-ubb-agent-id"
	timeout           = 60 * time.Second
	checkCacheTimeout = 60 * time.Second
)

type ServiceControlEndpoint struct {
	name        string
	serviceName string
	consumerId  string
	agentId     string
	keyData     string
	service     *servicecontrol.Service
	tracker     pipeline.UsageTracker
	nextCheck   time.Time
	clock       clock.Clock
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
	return newServiceControlEndpoint(name, serviceName, agentId, consumerId, service, clock.NewClock()), nil
}

func newServiceControlEndpoint(name, serviceName, agentId, consumerId string, service *servicecontrol.Service, clock clock.Clock) *ServiceControlEndpoint {
	ep := &ServiceControlEndpoint{
		name:        name,
		serviceName: serviceName,
		agentId:     agentId,
		consumerId:  consumerId,
		service:     service,
		clock:       clock,
	}
	return ep
}

func (ep *ServiceControlEndpoint) Name() string {
	return ep.name
}

func (ep *ServiceControlEndpoint) Send(report pipeline.EndpointReport) error {
	operation := ep.format(report)
	req := &servicecontrol.ReportRequest{
		Operations: []*servicecontrol.Operation{operation},
	}
	glog.V(2).Infoln("ServiceControlEndpoint:Send(): serviceName: ", ep.serviceName, " body: ", func() string {
		reqJson, _ := req.MarshalJSON()
		return string(reqJson)
	}())

	// Check only every 60 seconds, following recommendation from https://godoc.org/google.golang.org/api/servicecontrol/v1#ServicesService.Check
	if ep.clock.Now().After(ep.nextCheck) {
		// Check requests can not have user labels.
		opNoLabels := *operation
		opNoLabels.UserLabels = nil
		checkReq := &servicecontrol.CheckRequest{
			Operation: &opNoLabels,
		}
		_, err := ep.service.Services.Check(ep.serviceName, checkReq).Do()
		if err != nil && !googleapi.IsNotModified(err) {
			return err
		}
		ep.nextCheck = ep.clock.Now().Add(checkCacheTimeout)
	}

	_, err := ep.service.Services.Report(ep.serviceName, req).Do()
	if err != nil && !googleapi.IsNotModified(err) {
		return err
	}
	glog.V(2).Infoln("ServiceControlEndpoint:Send(): success")
	// TODO(volkman): Handle potential per-operation errors in response body
	return nil
}

func (ep *ServiceControlEndpoint) BuildReport(r metrics.StampedMetricReport) (pipeline.EndpointReport, error) {
	return pipeline.NewEndpointReport(r, nil)
}

func (ep *ServiceControlEndpoint) format(r pipeline.EndpointReport) *servicecontrol.Operation {
	value := servicecontrol.MetricValue{
		StartTime: r.StartTime.UTC().Format(time.RFC3339Nano),
		EndTime:   r.EndTime.UTC().Format(time.RFC3339Nano),
	}

	value.Int64Value = r.Value.Int64Value
	value.DoubleValue = r.Value.DoubleValue

	op := &servicecontrol.Operation{
		OperationId: r.Id,
		// ServiceControl requires this field but doesn't indicate what it's supposed to be.
		OperationName: fmt.Sprintf("%v/report", ep.serviceName),
		StartTime:     r.StartTime.UTC().Format(time.RFC3339Nano),
		EndTime:       r.EndTime.UTC().Format(time.RFC3339Nano),
		ConsumerId:    ep.consumerId,
		UserLabels:    r.Labels,
		MetricValueSets: []*servicecontrol.MetricValueSet{
			{
				MetricName:   fmt.Sprintf("%v/%v", ep.serviceName, r.Name),
				MetricValues: []*servicecontrol.MetricValue{&value},
			},
		},
	}

	if op.UserLabels == nil {
		op.UserLabels = make(map[string]string)
	}

	// Add the agent ID label
	op.UserLabels[agentIdLabel] = ep.agentId

	return op
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
	switch v := err.(type) {
	case *googleapi.Error:
		// Return true if this is an http error with a 5xx code.
		return v.Code >= 500 && v.Code < 600
	case net.Error:
		// Return true if this error is considered temporary or a timeout.
		return v.Temporary() || v.Timeout()
	default:
		// Some non-http error (perhaps a connection refused or timeout?)
		// We'll retry.
		return true
	}
}
