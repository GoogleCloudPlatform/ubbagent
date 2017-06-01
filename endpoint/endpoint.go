package endpoint

import "ubbagent/metrics"

// EndpointReport is an Endpoint-specific structure that contains a MetricBatch formatted for
// consumption by the remote service represented by the Endpoint. A report may contain additional
// information, such as a unique ID used for deduplication. The Dispatcher handles send failure
// retries, and may call the Endpoint's Send method multiple times with the same Report instance.
type EndpointReport interface{}

// Endpoint represents a metric reporting endpoint that the agent reports to. For example, Cloud
// Service Control or PubSub.
type Endpoint interface {
	// Name returns the name of this endpoint. The name must be unique across all endpoints in the
	// system, and should be constant across restarts of the agent. There can be multiple instances
	// of the same type of endpoint with different names.
	Name() string

	// Send sends an EndpointReport previously built by this Endpoint.
	Send(EndpointReport) error

	// BuildReport builds an EndpointReport from the given MetricBatch. The contents of the report
	// are specific to the endpoint.
	BuildReport(metrics.MetricBatch) (EndpointReport, error)

	// EmptyReport returns an empty EndpointReport structure and is used when loading previously
	// serialized reports from persistent state.
	EmptyReport() EndpointReport
}
