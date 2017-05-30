package endpoint

import "ubbagent/metrics"

// Report is an Endpoint-specific structure that contains a MetricBatch formatted for consumption
// by the remote service represented by the Endpoint. A report may contain additional information,
// such as a unique ID used for deduplication. The Dispatcher handles send failure retries, and may
// call the Endpoint's Send method multiple times with the same Report instance.
type Report interface{}

// Endpoint represents a metric reporting endpoint that the agent reports to. For example, Cloud
// Service Control or PubSub.
type Endpoint interface {
	Name() string
	BuildReport(metrics.MetricBatch) (Report, error)
	Send(Report) error
}
