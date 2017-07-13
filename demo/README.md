# Demo instructions

The demo will make use of three shells:
- shell1: Customer environment
- shell2: Marketplace environment
- shell3: Procurement service environment

Detailed instructions for procurement service and client here:
https://g3doc.corp.google.com/cloud/marketplace/whitebox_ubb/procurement/README.md?cl=head


## Setup

```
mkdir /tmp/ubbdemo
```

### shell1 (Customer)

Build and install ubbagent

```
cd ubbagent
make deps
make
go install ubbagent
```

### shell2 (Marketplace)

Build and alias the procurement client:

```
cd google3
export GOOGLE_APPLICATION_CREDENTIALS=<keyfile for cloud-marketplace-wb-ubb-stg@@appspot.gserviceaccount.com>
blaze build cloud/marketplace/whitebox_ubb/procurement/client/client
alias client="$PWD/blaze-bin/cloud/marketplace/whitebox_ubb/procurement/client/client"
```


### shell3 (Procurement)

Start local procurement server:

```
cd google3
export GOOGLE_APPLICATION_CREDENTIALS=<keyfile for cloud-marketplace-wb-ubb-stg@@appspot.gserviceaccount.com>
cloud/marketplace/whitebox_ubb/procurement/app.sh deploy dev
tail -f /tmp/dev_appserver_8080.log
```

## Demo script

shell1 (Customer): Examine config template.

```
cat demo/config.yaml.template
```

shell2 (Marketplace): Create entitlement.

```
client create $USER test-marketplace-whitebox-usage.googleapis.com | tee /tmp/ubbdemo/entitlement.json
```

shell2 (Marketplace): Create reporting key.

```
client create_key $(jq -r '.name' /tmp/ubbdemo/entitlement.json) | tee /tmp/ubbdemo/key.json
```

shell1 (Customer): Expand the config template.

```
export CONSUMER_ID="project:$(jq -r '.entitlement_name' /tmp/ubbdemo/key.json)"
export REPORTING_KEY="$(jq -r '.private_key_data' /tmp/ubbdemo/key.json | base64 --decode)"
envsubst < demo/config.yaml.template | tee /tmp/ubbdemo/config.yaml
```

shell1 (Customer): Start the agent in the background.

```
ubbagent --config /tmp/ubbdemo/config.yaml --no-state --local-port 3456 --logtostderr --v=2 &
```

shell1 (Customer): Send a report to the agent.

```
curl -X POST -d "{\"Name\": \"requests\", \"StartTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"EndTime\": \"$(date -u +"%Y-%m-%dT%H:%M:%S.%NZ")\", \"Value\": { \"IntValue\": 10 }, \"Labels\": { \"foo\": \"bar2\" } }" 'http://localhost:3456/report'
```

shell1 (Customer): Examine the local log.

```
head /tmp/ubbdemo/reports/*
```

Chemist -> Argentum dashboard:

https://pcon.corp.google.com/p#servicecontrol-billing/servicecontrol-billing%20pipeline/ingestion%20per%20service?group=&duration=4h&f:allmetrics:service=test-marketplace-whitebox-usage.googleapis.com
