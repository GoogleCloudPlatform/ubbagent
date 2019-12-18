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

package main

/*
#cgo pkg-config: python2
#define Py_LIMITED_API
#include <Python.h>
#include "api.h"

// Go cannot call C variadic functions. This simple wrapper allows us to call ParseTuple and expect
// two strings.
static int PyArg_ParseTuple_ss(PyObject *args, char **a, char **b) {
  return PyArg_ParseTuple(args, "ss", a, b);
}

// Go cannot use C macros directly. Py_RETURN_NONE is the standard mechanism for returning the
// PyNone object after incrementing its ref count.
static PyObject* none() {
  Py_RETURN_NONE;
}
*/
import "C"

import (
	"github.com/GoogleCloudPlatform/ubbagent/sdk"
	"sync"

)


// We store all current agents in a map keyed by an incrementing integer. Since the Python side of
// our module can't hold onto a Go reference, it instead holds onto the agent number which is
// subsequently used to retrieve the actual agent object when performing operations.
var agentCount C.int = 0
var agents = make(map[C.int]*sdk.Agent)

// A mutex that protects the agents map against concurrent modification.
var agentsmu = sync.RWMutex{}

//export AgentInit
func AgentInit(self *C.Agent, args *C.PyObject, _ *C.PyObject) C.int {
	var cConfigData *C.char
	var cStateDir *C.char

	if C.PyArg_ParseTuple_ss(args, &cConfigData, &cStateDir) == 0 {
		return -1
	}

	agentsmu.Lock()
	defer agentsmu.Unlock()

	num := agentCount
	agentCount++
	goConfigData := []byte(C.GoString(cConfigData))
	goStateDir := C.GoString(cStateDir)

	agent, err := sdk.NewAgent(goConfigData, goStateDir)
	if err != nil {
		setException(err.Error())
		return -1
	}

	agents[num] = agent
	self.agentnum = num

	return 0
}

//export AgentShutdown
func AgentShutdown(self *C.Agent, _ *C.PyObject) *C.PyObject {
	agentsmu.Lock()
	defer agentsmu.Unlock()

	agent, exists := agents[self.agentnum]
	if !exists {
		setException("Agent already shutdown")
		return nil
	}
	delete(agents, self.agentnum)

	err := agent.Shutdown()
	if err != nil {
		setException(err.Error())
		return nil
	}

	return C.none()
}

//export AgentDealloc
func AgentDealloc(self *C.Agent) {
	agentsmu.Lock()
	defer agentsmu.Unlock()

	agent, exists := agents[self.agentnum]
	if !exists {
		return
	}
	delete(agents, self.agentnum)

	// Ignore any shutdown errors
	agent.Shutdown()
}

//export AgentAddReport
func AgentAddReport(self *C.Agent, report *C.PyObject) *C.PyObject {
	var reportStr *C.PyObject = C.PyObject_Str(report)
	var reportData *C.char = C.PyString_AsString(reportStr)
	C.Py_DecRef(reportStr)

	agentsmu.RLock()
	defer agentsmu.RUnlock()

	goReportData := []byte(C.GoString(reportData))

	agent, exists := agents[self.agentnum]
	if !exists {
		setException("Agent already shutdown")
		return nil
	}

	if err := agent.AddReportJson(goReportData); err != nil {
		setException(err.Error())
		return nil
	}

	return C.none()
}

//export AgentGetStatus
func AgentGetStatus(self *C.Agent, _ *C.PyObject) *C.PyObject {
	agentsmu.RLock()
	defer agentsmu.RUnlock()

	agent, exists := agents[self.agentnum]
	if !exists {
		setException("Agent already shutdown")
		return nil
	}

	marshaled, err := agent.GetStatusJson()
	if err != nil {
		setException(err.Error())
		return nil
	}

	status := C.CString(string(marshaled))
	return C.PyString_FromString(status)
}

func setException(err string) {
	C.PyErr_SetString(C.AgentError, C.CString(err))
}

// Required empty func
func main() {}
