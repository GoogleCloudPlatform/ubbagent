module github.com/GoogleCloudPlatform/ubbagent

go 1.24.0

toolchain go1.24.1

require (
	github.com/ghodss/yaml v1.0.0
	github.com/golang/glog v1.2.5
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-multierror v1.1.1
	golang.org/x/oauth2 v0.34.0
	google.golang.org/api v0.124.0
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/s2a-go v0.1.4 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.2.3 // indirect
	github.com/googleapis/gax-go/v2 v2.8.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v2 v2.2.4 // indirect
)

replace golang.org/x/crypto => golang.org/x/crypto v0.35.0
