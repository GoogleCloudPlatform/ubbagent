# Metering agent

This metering agent simplifies usage metering of applications and can be used as part of a usage-based billing strategy. It performs the following functions:
* Accepts usage reports from a local source, such as an application processing requests
* Aggregates that usage and persists it across restarts
* Forwards usage to one or more endpoints, retrying in the case of failures

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
identities:
- name: gcp
  gcp:
    # A base64-encoded service account key used to report usage to
    # Google Service Control.
    encodedServiceAccountKey: [base64-encoded key]

# The metrics section defines the metric names and types that the agent
# is configured to record.
metrics:
- name: requests
  type: int

  # The endpoints section of a metric defines which endpoints the metric data is sent to.
  endpoints:
  - name: on_disk
  - name: servicecontrol

  # The aggregation section indicates that reports that the agent receives for this metric should
  # be aggregated for a specified period of time prior to being sent to the reporting endpoint.
  aggregation:
    bufferSeconds: 10

- name: instance-seconds
  type: int
  # The empty passthrough second indicates that no aggregation should occur for this metric.
  # Reports received are immediately sent to the reporting endpoint.
  passthrough: {}
  endpoints:
  - name: on_disk
  - name: servicecontrol

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
    identity: gcp
    serviceName: some-service-name.myapi.com
    consumerId: project:<project_id>

# The sources section lists metric data sources run by the agent itself. The currently-supported
# source is 'heartbeat', which sends a defined value to a metric at a defined interval.
sources:
- name: instance-seconds
  heartbeat:
    metric: instance-seconds
    intervalSeconds: 10
    value:
      int64Value: 10
    labels:
      auto: true
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
curl -X POST -d "{\"name\": \"requests\", \"startTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"endTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"value\": { \"int64Value\": 10 }, \"labels\": { \"foo\": \"bar2\" } }" 'http://localhost:3456/report'
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
See [DESIGN.md](doc/DESIGN.md).

# Kubernetes
The easiest way to deploy the metering agent into a Kubernetes cluster is as
a sidecar container alongside the software being metered. A Dockerfile is
provided that builds such a container. It accepts the following parameters
as environment variables:

* `AGENT_CONFIG_FILE` - Required. The path to a file containing the agent's
configuration.
* `AGENT_STATE_DIR` - Optional. The path under which the agent stores state.
If this parameter is not specified, no state will be stored.
* `AGENT_LOCAL_PORT` - Optional. The pod-local port on which the agent's
HTTP API will listen for reports and provide status. If this parameter
is not specified, the agent will not start its HTTP server.

The configuration file is run through envsubst, so it can contain
any additional parameters as well. For example, a service account
key and a servicecontrol consumerId may be stored in a Kubernetes
secret and passed in as environment variables.
