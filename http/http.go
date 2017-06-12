package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"ubbagent/metrics"
)

type HttpInterface struct {
	aggregator *metrics.Aggregator
	port       int
	mux        http.ServeMux
	srv        *http.Server
}

// NewHttpInterface creates a new agent interface that listens on the given port. The interface
// must be started with a call to ListenAndServe().
func NewHttpInterface(aggregator *metrics.Aggregator, port int) *HttpInterface {
	h := &HttpInterface{aggregator: aggregator, port: port}
	h.mux.HandleFunc("/report", h.handleAdd)
	return h
}

func (h *HttpInterface) handleAdd(w http.ResponseWriter, r *http.Request) {
	// TODO(volkman): better error handling (internal vs client errors)
	// TODO(volkman): request logging
	decoder := json.NewDecoder(r.Body)
	var report metrics.MetricReport
	if err := decoder.Decode(&report); err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	if err := h.aggregator.AddReport(report); err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Start starts the HttpInterface in the background. It returns an error immediately if background
// starting fails, but otherwise returns nil. The errHandler callback receives any errors returned
// by the underlying call to ListenAndServe(). Note that the background service may fail quickly
// after startup, such as in the case of a port already in use.
func (h *HttpInterface) Start(errHandler func(error)) error {
	if h.srv != nil {
		return errors.New("already started")
	}
	h.srv = &http.Server{Addr: fmt.Sprintf("localhost:%v", h.port), Handler: &h.mux}
	go func() {
		errHandler(h.srv.ListenAndServe())
	}()
	return nil
}

// Shutdown initiates a graceful shutdown of the HttpInterface and blocks until the operation
// finishes.
func (h *HttpInterface) Shutdown() error {
	if h.srv == nil {
		return errors.New("not started")
	}
	err := h.srv.Shutdown(context.Background())
	h.srv = nil
	return err
}
