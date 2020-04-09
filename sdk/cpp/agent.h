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

#include <memory>

#include "absl/status/status.h"
#include "absl/time/time.h"

namespace ubbagent {

struct AgentStatus {
    absl::Time last_report_success;
    int current_failure_count;
    int total_failure_count;
    absl::Status status;
};

// This class acts as a wrapper for the Go sdk. Creating this agent will create a Go agent.
// This class' destructor will deallocate the Go agent.
class Agent {
  public:
    // Factory method to create Agent. Result of the operation will be written to out_status.
    static std::unique_ptr<Agent> Create(const std::string& config,
                                         const std::string& state_dir,
                                         absl::Status* out_status);
    
    // Desctuctor shuts down the agent.
    ~Agent();

    // Adds a report to be sent.
    absl::Status AddReport(const std::string& report);

    // Gets the status of the agent and the reports it has sent or failed to send.
    AgentStatus GetStatus();

  private:
    // Private constructor because it could fail. Use the factory method to create Agent.
    Agent(const std::string& config, const std::string& state_dir, absl::Status* out_status);

    // This is an id that will be returned from Go ubbagent when the agent is first created.
    // Use this id when communicating with Go ubbagent.
    int id_ = -1;
};

} // namespace ubbagent
