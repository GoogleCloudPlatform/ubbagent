# Metering agent

This metering agent simplifies usage metering of applications and can be used as part of a usage-based billing strategy. It performs the following functions:
* Accepts usage reports from a local source, such as an application processing requests
* Aggregates that usage and persists it across restarts
* Ultimately forwards usage to one or more endpoints, retrying in the case of failures

# Build and run

```
git clone https://github.com/GoogleCloudPlatform/ubbagent.git
cd ubbagent
make setup deps build
bin/ubbagent --help
```

# Configuration

```yaml
# The identity section contains authentication information used by the agent.
identity:
  # A base64-encoded service account key used to report usage to
  # Google Service Control.
  encodedServiceAccountKey: [base64-encoded key]

# The metrics section defines the metric names and types that the agent
# is configured to record.
metrics:
  # bufferSeconds indicates how long values area aggregated prior to being sent to endpoints.
  bufferSeconds: 10
  definitions:
  - name: requests
    type: int
  - name: instance-seconds
    type: int

# The endpoints section defines where metering data is ultimately sent. Currently
# supported endpoints include:
# * disk - some directory on the local filesystem
# * servicecontrol - Google Service Control: https://cloud.google.com/service-control/overview
endpoints:
- name: on_disk
  disk:
    reportDir: /var/ubbagent/reports
    expireSeconds: 3600
- name: servicecontrol
  servicecontrol:
    serviceName: some-service-name.myapi.com
    consumerId: project:<project_id>
```

# Running

To run the agent, provide the following:
* A local TCP port (for the agent's HTTP daemon)
* The path to the agent's YAML config file
* The path to a directory used to store state

```
ubbagent --config path/to/config.yaml --state-dir path/to/state \
         --local-port 3456 --logtostderr --v=2
```

# Usage

The agent provides a local HTTP instance for interaction with metered software.
An example `curl` command to post a report:

```
curl -X POST -d "{\"Name\": \"requests\", \"StartTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"EndTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"Value\": { \"IntValue\": 10 }, \"Labels\": { \"foo\": \"bar2\" } }" 'http://localhost:3456/report'
```

The agent also provides status indicating its ability to send data to endpoints.

```
curl http://localhost:3456/status
{
  "lastReportSuccess": "2017-10-04T10:06:15.820953439-07:00",
  "currentFailureCount": 0,
  "totalFailureCount": 0
}
```

# Design
See [DESIGN.md](DESIGN.md).
