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

#include <chrono>
#include <ctime>
#include <fstream>
#include <json/value.h>
#include <json/reader.h>
#include <thread>

#include "absl/status/status.h"
#include "absl/strings/substitute.h"
#include "dirent.h"
#include "gtest/gtest.h"

namespace ubbagent {

namespace {

constexpr char kConfig[] = R"(
metrics:
- name: requests
  type: int
  aggregation:
    bufferSeconds: 1
  endpoints:
  - name: disk
endpoints:
- name: disk
  disk:
    reportDir: $0
    expireSeconds: 3600
)";

constexpr char kReportJson[] = R"(
{
    "name": "requests",
    "value": {
        "int64value": 25
    },
    "startTime": "1991-01-01T00:00:00Z",
    "endTime": "1992-01-01T00:00:00Z"
}
)";


// Returns true if <full_string> ends in <ending>.
bool HasEnding(const std::string& full_string, const std::string& ending) {
    if (full_string.length() >= ending.length()) {
        return (0 == full_string.compare (full_string.length() - ending.length(), ending.length(), ending));
    } else {
        return false;
    }
}

// Generates a random string to be used for the directory name.
std::string GenerateRandomString(const int length) {
    auto randchar = []() -> char {
        const char charset[] = "abcdefghijklmnopqrstuvwxyz";
        const size_t max_index = (sizeof(charset) - 1);
        return charset[ rand() % max_index ];
    };
    std::string str(length, 0);
    std::generate_n(str.begin(), length, randchar);
    return str;
}

// Adds up the number of request values written to disk by the agent.
int CountReportsOnDisk(const std::string& directory) {
    DIR *dir = opendir(directory.c_str());
    if (dir == NULL) {
      return 0;
    }
    struct dirent *ent;
    // The sum of values written to json files by the agent.
    int total_value = 0;
    while ((ent = readdir(dir)) != NULL) {
        if (!HasEnding(ent->d_name, ".json")) {
            // Only care about the reports which are json files.
            continue;
        }
        // Read the report.
        std::ifstream ifs(directory + ent->d_name);
        std::string content((std::istreambuf_iterator<char>(ifs)),
                       (std::istreambuf_iterator<char>()));
        // Parse the json.
        Json::Value root;   
        Json::Reader reader;
        if (!reader.parse(content.c_str(), root)) {
            // Unable to parse json.
            continue;
        }
        if (root.get("name", "").asString() != "requests") {
            // Only care about json files with the name "requests".
            continue;
        }
        if (!root.isMember("value")) {
            // Doesn't have "value".
            continue;
        }
        Json::Value value = root.get("value", "");
        total_value += value.get("int64Value", 0).asInt();
    }
    closedir(dir);
    return total_value;
}


class AgentTest : public ::testing::Test {
 protected:
  void SetUp() override {
    srand(time(NULL));
    directory_ = absl::StrCat("/tmp/ubbagent/report_", GenerateRandomString(15), "/");
    config_ = absl::Substitute(kConfig, directory_);
    directory_2_ = absl::StrCat("/tmp/ubbagent/report_", GenerateRandomString(15), "/");
    config_2_ = absl::Substitute(kConfig, directory_2_);
  }

  void TearDown() override {
    CleanDirectory(directory_);
    CleanDirectory(directory_2_);
  }

  // Directory where the agent will save reports.
  std::string directory_;
  std::string directory_2_;
  // UbbAgent config.
  std::string config_;
  std::string config_2_;

 private:
  // Removes any files that were saved to disk.
  void CleanDirectory(const std::string& directory) {
    DIR *dir = opendir(directory.c_str());
    if (dir == NULL) {
        return;
    }
    struct dirent *ent;
    while ((ent = readdir(dir)) != NULL) {
        std::string file_path = directory + ent->d_name;
        remove(file_path.c_str());
    }
    remove(directory.c_str());
    closedir(dir);
  }
};

TEST_F(AgentTest, CreateAgentFail) {
    absl::Status create_status;
    std::unique_ptr<Agent> agent = Agent::Create("bad_config", "", &create_status);
    EXPECT_FALSE(create_status.ok());
    EXPECT_EQ(agent, nullptr);
}

TEST_F(AgentTest, CreateAgentSuccess) {
    absl::Status create_status;
    std::unique_ptr<Agent> agent = Agent::Create(config_, "", &create_status);
    EXPECT_TRUE(create_status.ok());
    EXPECT_NE(agent, nullptr);
    // Shut down the agent.
    agent->Shutdown();
}

TEST_F(AgentTest, AddReportFail) {
    absl::Status create_status;
    std::unique_ptr<Agent> agent = Agent::Create(config_, "", &create_status);
    EXPECT_TRUE(create_status.ok());
    ASSERT_NE(agent, nullptr);

    // Fail to AddReport because of invalid json.
    absl::Status report_status = agent->AddReport("invalid_json");
    EXPECT_FALSE(report_status.ok());

    // Allow time for reports to be sent.
    std::this_thread::sleep_for(std::chrono::seconds(2));

    AgentStatus agent_status = agent->GetStatus();
    // Able to get the agent status.
    EXPECT_TRUE(agent_status.status.ok());
    // No reports attempted to be sent because AddReport received invalid json.
    // This also means no failures to send the report.
    EXPECT_LT(agent_status.last_report_success, absl::FromUnixSeconds(1));
    EXPECT_EQ(agent_status.current_failure_count, 0);
    EXPECT_EQ(agent_status.total_failure_count, 0);

    // Nothing reported.
    EXPECT_EQ(CountReportsOnDisk(directory_), 0);

    // Shut down the agent.
    agent->Shutdown();
}

TEST_F(AgentTest, AddReportSuccess) {
    // Create first agent.
    absl::Status create_status;
    std::unique_ptr<Agent> agent = Agent::Create(config_, "", &create_status);
    EXPECT_TRUE(create_status.ok());
    ASSERT_NE(agent, nullptr);

    // Create second agent.
    std::unique_ptr<Agent> agent_2 = Agent::Create(config_2_, "", &create_status);
    EXPECT_TRUE(create_status.ok());
    ASSERT_NE(agent_2, nullptr);

    // First agent send 3 reports.
    absl::Status report_status = agent->AddReport(kReportJson);
    EXPECT_TRUE(report_status.ok());
    report_status = agent->AddReport(kReportJson);
    EXPECT_TRUE(report_status.ok());
    report_status = agent->AddReport(kReportJson);
    EXPECT_TRUE(report_status.ok());

    // Second agent send 2 reports.
    report_status = agent_2->AddReport(kReportJson);
    EXPECT_TRUE(report_status.ok());
    report_status = agent_2->AddReport(kReportJson);
    EXPECT_TRUE(report_status.ok());

    // Allow time for reports to be sent.
    std::this_thread::sleep_for(std::chrono::seconds(2));

    AgentStatus agent_status = agent->GetStatus();
    // Able to get the first agent status.
    EXPECT_TRUE(agent_status.status.ok());
    // There should have been a successful report.
    EXPECT_GT(agent_status.last_report_success, absl::FromUnixSeconds(0));
    // There should be no errors.
    EXPECT_EQ(agent_status.current_failure_count, 0);
    EXPECT_EQ(agent_status.total_failure_count, 0);

    agent_status = agent_2->GetStatus();
    // Able to get the second agent status.
    EXPECT_TRUE(agent_status.status.ok());
    // There should have been a successful report.
    EXPECT_GT(agent_status.last_report_success, absl::FromUnixSeconds(0));
    // There should be no errors.
    EXPECT_EQ(agent_status.current_failure_count, 0);
    EXPECT_EQ(agent_status.total_failure_count, 0);

    // First agent sent 3 reports of 25 each.
    EXPECT_EQ(CountReportsOnDisk(directory_), 75);
    // Second agent sent 2 reports of 25 each.
    EXPECT_EQ(CountReportsOnDisk(directory_2_), 50);

    // Shut down the agents.
    agent->Shutdown();
    agent_2->Shutdown();
}

}  // namespace

} // namespace ubbagent

