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
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/clock"
	"github.com/GoogleCloudPlatform/ubbagent/metrics"
	"github.com/GoogleCloudPlatform/ubbagent/pipeline"
	"github.com/GoogleCloudPlatform/ubbagent/util"
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

type checkError struct {
	err       error
	transient bool
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
		checkResp, err := ep.service.Services.Check(ep.serviceName, checkReq).Do()
		if err != nil && !googleapi.IsNotModified(err) {
			return err
		}

		if len(checkResp.CheckErrors) > 0 {
			return checkErrorsToError(checkResp.CheckErrors)
		}

		ep.nextCheck = ep.clock.Now().Add(checkCacheTimeout)
	}

	resp, err := ep.service.Services.Report(ep.serviceName, req).Do()
	if err != nil && !googleapi.IsNotModified(err) {
		return err
	}

	// This will retry reporting all operations.
	// However, identical operations are de-duped for billing
	if len(resp.ReportErrors) > 0 {
		var errs []error
		for _, reportErr := range resp.ReportErrors {
			errs = append(errs, reportErrorToError(reportErr))
		}
		return errors.Join(errs...)
	}

	glog.V(2).Infoln("ServiceControlEndpoint:Send(): success")
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

	if r.Value.Int64Value != nil {
		value.Int64Value = util.NewInt64(*r.Value.Int64Value)
	} else if r.Value.DoubleValue != nil {
		value.DoubleValue = util.NewFloat64(*r.Value.DoubleValue)
	}

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
	case *checkError:
		return v.transient
	default:
		// Some non-http error (perhaps a connection refused or timeout?)
		// We'll retry.
		return true
	}
}

func checkErrorsToError(checkErrors []*servicecontrol.CheckError) error {
	var errs []error
	var transient = true
	for _, checkError := range checkErrors {
		fmt.Println("Check error", checkError.Code)
		switch checkError.Code {
		// These errors indicate customer disabling billing and
		// is not retriable. See: https://cloud.google.com/marketplace/docs/partners/integrated-saas/backend-integration#for_usage-based_pricing_reporting_usage_to_google
		case "BILLING_DISABLED", "SERVICE_NOT_ACTIVATED", "PROJECT_DELETED":
			transient = false
			fmt.Println("Transient")
		}
		bytes, _ := checkError.MarshalJSON()
		errs = append(errs, errors.New(string(bytes)))
	}
	return &checkError{err: errors.Join(errs...), transient: transient}
}

func (ce checkError) Error() string {
	return ce.err.Error()
}

func reportErrorToError(reportError *servicecontrol.ReportError) error {
	bytes, _ := reportError.MarshalJSON()
	return errors.New(string(bytes))
}
