package endpoint

import (
	"sync"
	"ubbagent/metrics"
)

// Dispatcher is a metrics.Sender that fans out to other Sender instances. Generally,
// this will be a collection of Endpoints wrapped in RetryingSender objects.
type Dispatcher struct {
	senders []metrics.Sender
}

func (d *Dispatcher) Prepare(mb metrics.MetricBatch) (metrics.PreparedSend, error) {
	sends := make([]metrics.PreparedSend, len(d.senders))
	for i, s := range d.senders {
		ps, err := s.Prepare(mb)
		if err != nil {
			return nil, err
		}
		sends[i] = ps
	}
	return &dispatcherSend{sends}, nil
}

type dispatcherSend struct {
	sends []metrics.PreparedSend
}

// Send fans out to each PreparedSend in parallel and returns the first error, if any. Send blocks
// until all sub-sends have finished.
func (ds *dispatcherSend) Send() error {
	results := make([]error, len(ds.sends))
	wg := sync.WaitGroup{}
	wg.Add(len(ds.sends))
	for i, ps := range ds.sends {
		go func(i int, ps metrics.PreparedSend) {
			results[i] = ps.Send()
			wg.Done()
		}(i, ps)
	}
	wg.Wait()
	for _, result := range results {
		// Return the first non-nil error.
		// TODO(volkman): return some sort of "multi error"?
		if result != nil {
			return result
		}
	}
	return nil
}

func NewDispatcher(senders []metrics.Sender) *Dispatcher {
	return &Dispatcher{senders}
}
