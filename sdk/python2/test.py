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
    self.reportDir = path.join(self.tempDir, 'reports')
    self.stateDir = path.join(self.tempDir, 'state')
    config = config_template.format(reportDir=self.reportDir)
    self.agent = ubbagent.Agent(config, self.stateDir)

  def tearDown(self):
    self.agent.shutdown()
    self.agent = None
    shutil.rmtree(self.tempDir)

  def testAgent(self):
    # Add some request reports
    for _ in range(10):
      self.agent.add_report(report_now('requests', 10))

    # Sleep to give the heartbeat time to send reports.
    time.sleep(2)

    # Now check output to make sure we see expected metric reports
    found_requests = False
    found_instance_time = False
    for report_file in os.listdir(self.reportDir):
      with open(path.join(self.reportDir, report_file)) as f:
        report = json.load(f)
      if report['name'] == 'requests' and report['value']['int64Value'] == 100:
        found_requests = True
      elif report['name'] == 'instance_time' and report[
        'value']['int64Value'] == 1:
        found_instance_time = True

    self.assertTrue(
        found_requests, 'Did not find a "requests" report with value 100')
    self.assertTrue(
        found_instance_time,
        'Did not find an "instance_time" report with value 1')


if __name__ == '__main__':
  unittest.main()
