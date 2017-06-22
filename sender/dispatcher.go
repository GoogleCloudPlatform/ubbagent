package sender

import (
	"github.com/hashicorp/go-multierror"
	"sync"
	"ubbagent/metrics"
)

// Dispatcher is a Sender that fans out to other Sender instances. Generally,
// this will be a collection of Endpoints wrapped in RetryingSender objects.
type Dispatcher struct {
	senders []Sender
}

// See Sender.Prepare.
func (d *Dispatcher) Prepare(mb metrics.MetricBatch) (PreparedSend, error) {
	sends := make([]PreparedSend, len(d.senders))
	for i, s := range d.senders {
		ps, err := s.Prepare(mb)
		if err != nil {
			return nil, err
		}
		sends[i] = ps
	}
	return &dispatcherSend{sends}, nil
}

// Close closes all of the underlying senders concurrently and waits for them all to finish.
// See pipeline.PipelineComponent.Close.
func (d *Dispatcher) Close() error {
	errors := make([]error, len(d.senders))
	wg := sync.WaitGroup{}
	wg.Add(len(d.senders))
	for i, s := range d.senders {
		go func(i int, s Sender) {
			errors[i] = s.Close()
			wg.Done()
		}(i, s)
	}
	wg.Wait()
	return multierror.Append(nil, errors...).ErrorOrNil()
}

type dispatcherSend struct {
	sends []PreparedSend
}

// Send fans out to each PreparedSend in parallel and returns the first error, if any. Send blocks
// until all sub-sends have finished.
func (ds *dispatcherSend) Send() error {
	errors := make([]error, len(ds.sends))
	wg := sync.WaitGroup{}
	wg.Add(len(ds.sends))
	for i, ps := range ds.sends {
		go func(i int, ps PreparedSend) {
			errors[i] = ps.Send()
			wg.Done()
		}(i, ps)
	}
	wg.Wait()
	return multierror.Append(nil, errors...).ErrorOrNil()
}

func NewDispatcher(senders []Sender) *Dispatcher {
	return &Dispatcher{senders}
}
