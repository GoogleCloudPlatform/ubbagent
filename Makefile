all: test

deps:
	go get -d -v github.com/golang/glog

updatedeps:
	go get -d -v -u github.com/golang/glog

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
