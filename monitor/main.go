/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/go-kit/log/level"
	"github.com/openshift-kni/eapol-operator/internal/logging"
	"github.com/openshift-kni/eapol-operator/pkg/hostap"
	"github.com/openshift-kni/eapol-operator/pkg/netlink"
)

func main() {
	var (
		interfaces = flag.String("interfaces", os.Getenv("IFACES"), "Interfaces on which hostapd to listen on")
		logLevel   = flag.String("log-level", "info", fmt.Sprintf("log level. must be one of: [%s]", logging.Levels.String()))
	)
	flag.Parse()

	logger, err := logging.Init(*logLevel)
	if err != nil {
		fmt.Printf("failed to initialize logging: %s\n", err)
		os.Exit(1)
	}

	if interfaces == nil || *interfaces == "" {
		level.Error(logger).Log("op", "startup", "error", "IFACES env variable must be set", "msg", "missing configuration")
		os.Exit(1)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT)
	done := make(chan bool, 1)

	goMaxProcs := os.Getenv("GOMAXPROCS")
	if goMaxProcs == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	ifEventHandler := netlink.LinkEventHandler{Logger: logger}
	ifEventHandler.Start()

	var monitors []*hostap.InterfaceMonitor
	for _, intf := range strings.Split(*interfaces, ",") {
		level.Info(logger).Log("op", "startup", "monitor start for interface", intf)
		intfMonitor := &hostap.InterfaceMonitor{Logger: logger, IfName: intf,
			IfEventHandler: ifEventHandler}
		err = intfMonitor.StartMonitor()
		if err != nil {
			level.Error(logger).Log("op", "startup", "start monitor on interface failed", intf, "error", err)
			continue
		}
		monitors = append(monitors, intfMonitor)
	}

	go func() {
		<-sigs
		level.Info(logger).Log("op", "shutdown", "msg", "starting shutdown")
		done <- true
	}()
	// Capture signals to cleanup before exiting
	<-done
	close(done)
	for _, monitor := range monitors {
		monitor.StopMonitor()
	}
	ifEventHandler.StopHandler()
	level.Info(logger).Log("op", "shutdown", "msg", "done")
}
