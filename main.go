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

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	httplib "net/http"
	"os"
	"os/signal"

	"github.com/GoogleCloudPlatform/ubbagent/http"
	"github.com/GoogleCloudPlatform/ubbagent/sdk"
	"github.com/golang/glog"
)

var configPath = flag.String("config", "", "configuration file")
var stateDir = flag.String("state-dir", "", "persistent state directory")
var noState = flag.Bool("no-state", false, "do not store persistent state")
var localPort = flag.Int("local-port", 0, "local HTTP daemon port")
var noHttp = flag.Bool("no-http", false, "do not start the HTTP daemon")

// main is the entry point to the standalone agent. It constructs a new app.App with the config file
// specified using the --config flag, and it starts the http interface. SIGINT will initiate a
// graceful shutdown.
func main() {
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "configuration file must be specified")
		flag.Usage()
		os.Exit(2)
	}

	if *stateDir == "" && !*noState {
		fmt.Fprintln(os.Stderr, "state directory must be specified (or use --no-state)")
		flag.Usage()
		os.Exit(2)
	}

	if *localPort <= 0 && !*noHttp {
		fmt.Fprintln(os.Stderr, "local-port must be > 0 (or use --no-http)")
		flag.Usage()
		os.Exit(2)
	}

	configData, err := ioutil.ReadFile(*configPath)
	if err != nil {
		exitf("startup: failed to read configuration file: %+v", err)
	}

	agent, err := sdk.NewAgent(configData, *stateDir)
	if err != nil {
		exitf("startup: failed to create agent: %+v", err)
	}

	var rest *http.HttpInterface
	if *localPort > 0 {
		rest = http.NewHttpInterface(agent, *localPort)
		if err := rest.Start(func(err error) {
			// Process async http errors (which may be an immediate port in use error).
			if err != httplib.ErrServerClosed {
				exitf("http: %+v", err)
			}
		}); err != nil {
			exitf("startup: %+v", err)
		}
		infof("Listening locally on port %v", *localPort)
	} else {
		infof("Not starting HTTP daemon")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c

	infof("Shutting down...")
	if rest != nil {
		rest.Shutdown()
	}
	if err := agent.Shutdown(); err != nil {
		glog.Warningf("shutdown: %+v", err)
	}
	glog.Flush()
}

// infof prints a message to stdout and also logs it to the INFO log.
func infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	glog.Info(msg)
}

// exitf prints a message to stderr, logs it to the FATAL log, and exits.
func exitf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintln(os.Stderr, msg)
	glog.Exit(msg)
}
