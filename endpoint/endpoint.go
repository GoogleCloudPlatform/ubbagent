package endpoint

import "ubbagent/metrics"

// Endpoint represents a metric reporting endpoint that the agent reports to. For example, Cloud
// Service Control or PubSub.
type Endpoint interface {
	Name() string
	Send([]metrics.MetricReport)
}
