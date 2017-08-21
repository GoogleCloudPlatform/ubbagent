# Usage-based billing agent

This directory contains a small agent intended to simplify usage-based billing of applications. The
agent starts a small, local HTTP daemon that accepts metering reports from software running on the
same host. Upon receiving a report, the agent:
* Aggregates the report with other reports received in close proximity
(buffering time is configurable)
* Stores the updated aggregate to persistent state in case the agent is killed or restarted
* Eventually forwards the aggregated report to one or more destination endpoints, such as Google
Service Control

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
  # A service account key retrieved from Google Cloud's console or API.
  # This key is generally provided in JSON format and can be embedded directly
  # into the configuration file.
  serviceAccountKey:
  {
    "type": "service_account",
    "project_id": "<project_id>",
    "private_key_id": "<private_key_id>",
    "private_key": "<private_key>",
    client_email": "<client_email>",
    client_id": "<client_id>",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://accounts.google.com/o/oauth2/token",
    "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
    "client_x509_cert_url": "<client_x509_cert_url>"
  }

# The metrics section defines the metric names and types that the agent
# is configured to record.
metrics:
  bufferSeconds: 10
  definitions:
  - name: requests
    billingName: com.googleapis/services/some-service-name/Requests
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
ubbagent --config path/to/config.yaml --state-dir path/to/state --local-port 3456 --logtostderr --v=2
```

# Usage

The agent provides a local HTTP instance for interaction with metered software.
An example `curl` command to post a report:

```
curl -X POST -d "{\"Name\": \"requests\", \"StartTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"EndTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"Value\": { \"IntValue\": 10 }, \"Labels\": { \"foo\": \"bar2\" } }" 'http://localhost:3456/report'
```
