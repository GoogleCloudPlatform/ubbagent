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

#include "sdk/cpp/agent.h"

#include <iostream>

#include "sdk/cpp/api.h"

namespace ubbagent {

Agent::Agent(const std::string& config, const std::string& state_dir, absl::Status* out_status) {
    // Copy the input strings because we need non-const char*.
    char c_config[config.size() + 1];
    strcpy(c_config, config.c_str());
    char c_state_dir[state_dir.size() + 1];
    strcpy(c_state_dir, state_dir.c_str());
    // Create a new agent.
    struct InitResult init_result = AgentInit(c_config, c_state_dir);
    if (init_result.error_message) {
        *out_status = absl::InternalError(std::string(init_result.error_message));
    } else {
        *out_status = absl::OkStatus();
        id_ = init_result.id;
    }
    free(init_result.error_message);
}


std::unique_ptr<Agent> Agent::Create(const std::string& config, const std::string& state_dir, absl::Status* out_status) {
    // The constructor is private. Use "new".
    std::unique_ptr<Agent> agent = absl::WrapUnique(new Agent(config, state_dir, out_status));
    if (!out_status->ok()) {
        // The agent is not usable. Return a nullptr.
        return nullptr;
    }
    return agent;
}


Agent::~Agent() {
    if (id_ == -1) {
        // Agent was never initialized.
        return;
    }
    // Shut down the agent.
    struct Result result = AgentShutdown(id_);
    free(result.error_message);
}


absl::Status Agent::AddReport(const std::string& report) {
    char c_report[report.size() + 1];
    strcpy(c_report, report.c_str());
    struct Result result = AgentAddReport(id_, c_report);
    absl::Status status;
    if (result.error_message) {
        status = absl::InternalError(std::string(result.error_message));
    } else {
        status = absl::OkStatus();
    }
    free(result.error_message);
    return status;
}


AgentStatus Agent::GetStatus() {
    struct CurrentStatus current_status = AgentGetStatus(id_);
    AgentStatus agent_status;
    if (current_status.error_message) {
        agent_status.status = absl::InternalError(std::string(current_status.error_message));
    } else {
        agent_status.status = absl::OkStatus();
        agent_status.last_report_success = absl::FromUnixSeconds(current_status.last_report_success);
        agent_status.current_failure_count = current_status.current_failure_count;
        agent_status.total_failure_count = current_status.total_failure_count;
    }
    free(current_status.error_message);
    return agent_status;
}

} // namespace ubbagent
