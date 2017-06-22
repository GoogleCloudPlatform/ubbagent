# TODO(volkman): Use a dependency management system (e.g., govendor)
all: test

deps:
	go get -d -v github.com/golang/glog
	go get -d -v gopkg.in/yaml.v2
	go get -d -v github.com/ghodss/yaml
	go get -d -v github.com/hashicorp/go-multierror

updatedeps:
	go get -d -v -u github.com/golang/glog
	go get -d -v -u gopkg.in/yaml.v2
	go get -d -v -u github.com/ghodss/yaml
	go get -d -v -u github.com/hashicorp/go-multierror

testdeps:

updatetestdeps:

build: deps
	go build ubbagent/...

test: testdeps
	go test -v -cpu 1,4 ubbagent/...

clean:
	go clean -i ubbagent/...

.PHONY: \
	all \
	deps \
	updatedeps \
	testdeps \
	updatetestdeps \
	build \
	test \
	clean \
