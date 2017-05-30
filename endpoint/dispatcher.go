package endpoint

import "ubbagent/metrics"

// Dispatcher sends a batch of MetricReports to configured Endpoints. It handles per-Endpoint
// buffering and retries.
type Dispatcher struct {
}

func (d *Dispatcher) Send(mb metrics.MetricBatch) {
	// TODO(volkman): implement
}
