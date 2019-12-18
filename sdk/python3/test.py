# Copyright 2018 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import datetime
import json
import os
from os import path
import shutil
import tempfile
import time
import unittest

import ubbagent

# Metering agent config with:
# * A 'requests' metric
# * An 'instance_time' metric
# * A heartbeat with a 1-second interval
# * A disk endpoint, writing to a temp directory.
config_template = '''
metrics:
- name: requests
  type: int
  aggregation:
    bufferSeconds: 1
  endpoints:
  - name: disk
- name: instance_time
  type: int
  passthrough: {{}}
  endpoints:
  - name: disk
endpoints:
- name: disk
  disk:
    reportDir: {reportDir}
    expireSeconds: 3600
sources:
- name: instance_time
  heartbeat:
    metric: instance_time
    intervalSeconds: 1
    value:
      int64Value: 1
'''


def report_now(name, value):
  '''Returns a metric report as a JSON string with the given name and value.'''
  report_time = datetime.datetime.utcnow().isoformat() + 'Z'
  return json.dumps({
    'name': name,
    'startTime': report_time,
    'endTime': report_time,
    'value': {
      'int64value': value,
    },
  })


class AgentSdkTest(unittest.TestCase):
  def setUp(self):
    self.tempDir = tempfile.mkdtemp('ubbagent-test')
    self.reportDir1 = path.join(self.tempDir, 'agent1', 'reports')
    self.stateDir1 = path.join(self.tempDir, 'agent1', 'state')
    self.reportDir2 = path.join(self.tempDir, 'agent2', 'reports')
    self.stateDir2 = path.join(self.tempDir, 'agent2', 'state')
    config1 = config_template.format(reportDir=self.reportDir1)
    config2 = config_template.format(reportDir=self.reportDir2)
    self.agent1 = ubbagent.Agent(config1, self.stateDir1)
    self.agent2 = ubbagent.Agent(config2, self.stateDir2)

  def tearDown(self):
    self.agent1.shutdown()
    self.agent1 = None
    self.agent2.shutdown()
    self.agent2 = None
    shutil.rmtree(self.tempDir)

  def testAgent(self):
    # Add some request reports
    for _ in range(10):
      self.agent1.add_report(report_now('requests', 10))   # 10*10 = 100 reqs
      self.agent2.add_report(report_now('requests', 100))  # 10*100 = 1000 reqs

    # Sleep to give the heartbeat time to send reports.
    time.sleep(2)

    # Now check output to make sure we see expected metric reports
    found_requests1 = False
    found_requests2 = False
    found_instance_time1 = False
    for report_file in os.listdir(self.reportDir1):
      with open(path.join(self.reportDir1, report_file)) as f:
        report = json.load(f)
      if report['name'] == 'requests' and report['value']['int64Value'] == 100:
        found_requests1 = True
      elif report['name'] == 'instance_time' and report[
        'value']['int64Value'] == 1:
        found_instance_time1 = True

    for report_file in os.listdir(self.reportDir2):
      with open(path.join(self.reportDir2, report_file)) as f:
        report = json.load(f)
      if report['name'] == 'requests' and report['value']['int64Value'] == 1000:
        found_requests2 = True

    self.assertTrue(
        found_requests1,
        'Did not find a "requests" report for agent 1 with value 100')
    self.assertTrue(
        found_instance_time1,
        'Did not find an "instance_time" report for agent 1 with value 1')
    self.assertTrue(
        found_requests2,
        'Did not find a "requests" report for agent 2 with value 1000')


if __name__ == '__main__':
  unittest.main()
