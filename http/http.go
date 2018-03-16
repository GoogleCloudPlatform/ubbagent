// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/GoogleCloudPlatform/ubbagent/sdk"
)

type HttpInterface struct {
	agent *sdk.Agent
	port  int
	mux   http.ServeMux
	srv   *http.Server
}

// NewHttpInterface creates a new agent interface that listens on the given port. The interface
// must be started with a call to ListenAndServe().
func NewHttpInterface(agent *sdk.Agent, port int) *HttpInterface {
	h := &HttpInterface{agent: agent, port: port}
	h.mux.HandleFunc("/report", h.handleAdd)
	h.mux.HandleFunc("/status", h.handleStatus)
	return h
}

func (h *HttpInterface) handleAdd(w http.ResponseWriter, r *http.Request) {
	// TODO(volkman): better error handling (internal vs client errors)
	// TODO(volkman): request logging

	reportData, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	err = h.agent.AddReportJson(reportData)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *HttpInterface) handleStatus(w http.ResponseWriter, r *http.Request) {
	text, err := h.agent.GetStatusJson()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write(text)
	}
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
