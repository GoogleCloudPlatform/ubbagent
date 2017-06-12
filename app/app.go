package app

import (
	"errors"
	"github.com/golang/glog"
	"time"
	"ubbagent/config"
	"ubbagent/endpoint"
	"ubbagent/endpoint/disk"
	"ubbagent/metrics"
	"ubbagent/persistence"
)

// closer is any type with a Close() method that returns an error. App will close these types during
// shutdown.
type closer interface {
	Close() error
}

// App represents an embedded agent (no HTTP interface). It contains the aggregator, the start of
// the reporting pipeline. It provides graceful shutdown ability, and will manage shutting down the
// various components behind the aggregator in the proper order.
type App struct {
	Aggregator *metrics.Aggregator

	// A list of things to close, in the order they should be closed.
	closers []closer
}

// NewApp creates a new App structure that contains a configured Aggregator and all of the resources
// (persistence, endpoints) behind it.
//
// TODO(volkman): Move this to a method in Aggregator (or similar), and make aggregator and the rest
// of the reporting pipeline components cascade during shutdown.
func NewApp(cfg *config.Config, p persistence.Persistence) (*App, error) {
	endpoints, err := createEndpoints(cfg)
	if err != nil {
		return nil, err
	}
	senders := make([]metrics.Sender, len(endpoints))
	for i := range endpoints {
		senders[i] = endpoint.NewRetryingSender(endpoints[i], p)
	}
	d := endpoint.NewDispatcher(senders)

	agg := metrics.NewAggregator(cfg.Metrics, d, p)

	// Construct the list of things to close at shutdown.
	var closers []closer

	// First, the Aggregator.
	closers = append(closers, agg)

	// Next, all of the RetryingSenders.
	for i := range senders {
		if s, ok := senders[i].(closer); ok {
			closers = append(closers, s)
		}
	}

	// Last, any Endpoints that need to be closed.
	for i := range endpoints {
		if ep, ok := endpoints[i].(closer); ok {
			closers = append(closers, ep)
		}
	}

	return &App{Aggregator: agg, closers: closers}, nil
}

// Shutdown gracefully terminates the app.
func (a *App) Shutdown() {
	for _, c := range a.closers {
		if err := c.Close(); err != nil {
			glog.Errorf("shutdown: %+v", err)
		}
	}
	a.closers = nil
}

func createEndpoints(config *config.Config) ([]endpoint.Endpoint, error) {
	var eps []endpoint.Endpoint
	for _, cfgep := range config.Endpoints {
		ep, err := createEndpoint(cfgep)
		if err != nil {
			// TODO(volkman): close already-created endpoints in event of error?
			return nil, err
		}
		eps = append(eps, ep)
	}
	return eps, nil
}

func createEndpoint(cfgep config.Endpoint) (endpoint.Endpoint, error) {
	if cfgep.Disk != nil {
		return disk.NewDiskEndpoint(cfgep.Name, cfgep.Disk.ReportDir, time.Duration(cfgep.Disk.ExpireSeconds)*time.Second), nil
	}
	// TODO(volkman): support servicecontrol and pubsub
	return nil, errors.New("unsupported endpoint")
}
