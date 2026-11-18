package main

//
// Installation instruction (golang tools are required):
// $ make setup
// $ make deps
// $ make build
//
// Sample command:
// $ bin/ubbagent -logtostderr \
//   -reporting-secret fake_reporting_secret.yaml \
//   -service-name your-solution.your-service-id.appspot.com
//   -metric-name your-metric-name \
//   -metric-int-value 1
//
// The service name and metric name are configured when billing
// is setup for your listing.
//
// If you deploy Cloud Bees Core Billable solution from Marketplace
// and obtain its reporting secret (look for *-reporting-secret Secret
// in the target namespace), you can use the following:
// $ bin/ubbagent -logtostderr \
//   -reporting-secret reporting_secret.yaml \
//   -service-name cloudbees-core-billable.mp-cloudbees.appspot.com \
//   -metric-name user \
//   -metric-int-value 1
//

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/GoogleCloudPlatform/ubbagent/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/servicecontrol/v1"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/google/uuid"
)

var secretPath = flag.String("reporting-secret", "", "K8s reporting secret YAML")
var serviceName = flag.String("service-name", "", "Service name")
var metricName = flag.String("metric-name", "", "Metric name")
var metricValue = flag.Int64("metric-int-value", 1, "Metric int64 value")

type secret struct {
	Data secretData `json:"data"`
}

type secretData struct {
	ConsumerID    consumerID    `json:"consumer-id"`
	EntitlementID entitlementID `json:"entitlement-id"`
	ReportingKey  reportingKey  `json:"reporting-key"`
}

type consumerID string

func (c *consumerID) UnmarshalJSON(data []byte) error {
	decoded, err := decodeBase64Encoded(data)
	if err != nil {
		return err
	}
	*c = consumerID(decoded)
	return nil
}

type entitlementID string

func (c *entitlementID) UnmarshalJSON(data []byte) error {
	decoded, err := decodeBase64Encoded(data)
	if err != nil {
		return err
	}
	*c = entitlementID(decoded)
	return nil
}

type reportingKey config.EncodedServiceAccountKey

func (c *reportingKey) UnmarshalJSON(data []byte) error {
	decoded, err := decodeBase64Encoded(data)
	if err != nil {
		return err
	}
	if decoded == nil {
		return err
	}
	accountKey := config.EncodedServiceAccountKey{}
	accountKey.UnmarshalJSON(decoded)
	*c = reportingKey(accountKey)
	return nil
}

func decodeBase64Encoded(data []byte) ([]byte, error) {
	var decoded []byte
	var encodedStr string

	// First we decode the data into a string to get rid of any start and end quotes.
	err := yaml.Unmarshal(data, &encodedStr)
	if err != nil {
		return nil, errors.New("not a string value")
	}

	decoded, err = base64.StdEncoding.DecodeString(encodedStr)
	if err != nil {
		return nil, errors.New("not a valid base64 value")
	}

	return decoded, nil
}

func main() {
	flag.Parse()

	if *secretPath == "" || *serviceName == "" || *metricName == "" {
		fmt.Fprintln(os.Stderr, "Required flags must be specified")
		flag.Usage()
		os.Exit(2)
	}
	reportingSecret, err := load(*secretPath)
	check(err)
	glog.Infof("ReportingSecret=%+v", reportingSecret)
	service, err := newServiceControl(reportingSecret.Data.ReportingKey)
	check(err)

	opID, err := uuid.NewRandom()
	check(err)
	op := &servicecontrol.Operation{
		OperationId: opID.String(),
		// ServiceControl requires this field but doesn't indicate what it's supposed to be.
		OperationName: fmt.Sprintf("%v/report", *serviceName),
		StartTime:     time.Now().Format(time.RFC3339Nano),
		EndTime:       time.Now().Format(time.RFC3339Nano),
		ConsumerId:    string(reportingSecret.Data.ConsumerID),
		MetricValueSets: []*servicecontrol.MetricValueSet{
			{
				MetricName: fmt.Sprintf("%v/%v", *serviceName, *metricName),
				MetricValues: []*servicecontrol.MetricValue{
					&servicecontrol.MetricValue{
						StartTime:  time.Now().Format(time.RFC3339Nano),
						EndTime:    time.Now().Format(time.RFC3339Nano),
						Int64Value: &[]int64{*metricValue}[0],
					},
				},
			},
		},
	}
	req := &servicecontrol.ReportRequest{
		Operations: []*servicecontrol.Operation{op},
	}
	response, err := service.Services.Report(*serviceName, req).Do()
	glog.Infof("Response=%v\nError=%v", response, err)
	if err == nil {
		glog.Infof("SUCCESS!!!")
	} else {
		glog.Fatalf("DID NOT SUCCEED")
	}
}

func check(err error) {
	if err != nil {
		glog.Fatalf("Error: %v", err)
	}
}

func newServiceControl(jsonKey []byte) (*servicecontrol.Service, error) {
	config, err := google.JWTConfigFromJSON(jsonKey, servicecontrol.ServicecontrolScope)
	if err != nil {
		return nil, err
	}
	client := config.Client(context.Background())
	client.Timeout = 30 * time.Second
	service, err := servicecontrol.New(client)
	if err != nil {
		return nil, err
	}
	return service, nil
}

func load(path string) (*secret, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	secret, err := parse(data)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func parse(data []byte) (*secret, error) {
	c := &secret{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}
