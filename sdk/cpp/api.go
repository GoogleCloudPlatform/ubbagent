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
struct InitResult {
	// If the error_message is a nullptr then the operation was a success. 
	// If not a nullptr, then error_message contains the error.
	char* error_message;
	// The id of the agent. This id should be used in future requests.
	int id;
};

struct Result {
	// If the error_message is a nullptr then the operation was a success. 
	// If not a nullptr, then error_message contains the error.
	char* error_message;
};

struct CurrentStatus {
	int current_failure_count;
	int total_failure_count;
	// Unix time UTC
	long last_report_success;
	// error_message indicates whether there was an error getting the status of the ubbagent. 
	char* error_message;
};
*/
import "C"

import (
	"github.com/GoogleCloudPlatform/ubbagent/sdk"
	"sync"
)

// We store all current agents in a map keyed by an incrementing integer. Since the c++ side of
// our module can't hold onto a Go reference, it instead holds onto the agent number which is
// subsequently used to retrieve the actual agent object when performing operations.
var agentCount C.int = 0
var agents = make(map[C.int]*sdk.Agent)

// A mutex that protects the agents map against concurrent modification.
var agentsmu = sync.RWMutex{}

//export AgentInit
func AgentInit(config *C.char, state_dir *C.char) C.struct_InitResult {
	agentsmu.Lock()
	defer agentsmu.Unlock()

	num := agentCount
	agentCount++
	goConfig := []byte(C.GoString(config))
	goStateDir := C.GoString(state_dir)

	agent, err := sdk.NewAgent(goConfig, goStateDir)
	if err != nil {
		return C.struct_InitResult{ error_message: C.CString(err.Error()) }
	}

	agents[num] = agent

	return C.struct_InitResult{ id: num }
}

//export AgentShutdown
func AgentShutdown(agent_id C.int) C.struct_Result {
	agentsmu.Lock()
	defer agentsmu.Unlock()

	agent, exists := agents[agent_id]
	if !exists {
		return C.struct_Result{ error_message: C.CString("Agent already shutdown") }
	}
	delete(agents, agent_id)

	err := agent.Shutdown()
	if err != nil {
		return C.struct_Result{ error_message: C.CString(err.Error()) }
	}

	// Agent was shut down successfully.
	return C.struct_Result{}
}


//export AgentAddReport
func AgentAddReport(agent_id C.int, report *C.char) C.struct_Result {
	agentsmu.RLock()
	defer agentsmu.RUnlock()

	goReportData := []byte(C.GoString(report))

	agent, exists := agents[agent_id]
	if !exists {
		return C.struct_Result{ error_message: C.CString("Agent does not exist") }
	}

	if err := agent.AddReportJson(goReportData); err != nil {
		return C.struct_Result{ error_message: C.CString(err.Error()) }
	}

	// Added report successfully.
	return C.struct_Result{}
}


//export AgentGetStatus
func AgentGetStatus(agent_id C.int) C.struct_CurrentStatus {
	agentsmu.RLock()
	defer agentsmu.RUnlock()

	agent, exists := agents[agent_id]
	if !exists {
		return C.struct_CurrentStatus{ error_message: C.CString("Agent does not exist") }
	}

	stats := agent.GetStatus()

	return C.struct_CurrentStatus{ current_failure_count: C.int(stats.CurrentFailureCount),
								   total_failure_count: C.int(stats.TotalFailureCount),
								   last_report_success: C.long(stats.LastReportSuccess.Unix()) }
}

// Required empty func
func main() {}
