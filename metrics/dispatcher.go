package metrics

// Dispatcher sends a batch of MetricReports to configured Endpoints. It handles per-Endpoint
// buffering and retries.
type Dispatcher struct {
	// TODO(volkman): implement
}

func (d *Dispatcher) Send(mrs []MetricReport) {
}
