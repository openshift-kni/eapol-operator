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
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	"github.com/openshift-kni/eapol-operator/internal/k8s"
	"github.com/openshift-kni/eapol-operator/internal/logging"
	"github.com/openshift-kni/eapol-operator/internal/trafficcontrol"
	"github.com/openshift-kni/eapol-operator/pkg/hostap"
	"github.com/openshift-kni/eapol-operator/pkg/netlink"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	var (
		interfaces          = flag.String("interfaces", os.Getenv("IFACES"), "Interfaces on which hostapd to listen on")
		unprotectedTcpPorts = flag.String("unprotected-tcp-ports", os.Getenv("UNPROTECTED_TCP_PORTS"), "list of unprotected tcp ports")
		unprotectedUdpPorts = flag.String("unprotected-udp-ports", os.Getenv("UNPROTECTED_UDP_PORTS"), "list of unprotected udp ports")
		logLevel            = flag.String("log-level", "info", fmt.Sprintf("log level. must be one of: [%s]", logging.Levels.String()))
		host                = flag.String("host", os.Getenv("AUTHENTICATOR_HOST"), "HTTP host address")
		port                = flag.Int("port", 7472, "HTTP listening port")
		enablePprof         = flag.Bool("enable-pprof", false, "Enable pprof profiling")
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
	ifaces := parseStringsArgs(interfaces)

	allowedTcpPorts, err := parseIntArgs(unprotectedTcpPorts)
	if err != nil {
		level.Error(logger).Log("op", "startup", "error", "UNPROTECTED_TCP_PORTS env variable must be set properly", "msg", "incorrect configuration")
		os.Exit(1)
	}
	allowedUdpPorts, err := parseIntArgs(unprotectedUdpPorts)
	if err != nil {
		level.Error(logger).Log("op", "startup", "error", "UNPROTECTED_UDP_PORTS env variable must be set properly", "msg", "incorrect configuration")
		os.Exit(1)
	}
	authObjKey, err := k8s.GetAuthNamespacedName()
	if err != nil {
		level.Error(logger).Log("op", "startup", "auth", "retrieval failed", "error", err)
		os.Exit(1)
	}
	level.Info(logger).Log("op", "startup", "authObjKey", authObjKey)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGQUIT)
	done := make(chan bool, 1)

	goMaxProcs := os.Getenv("GOMAXPROCS")
	if goMaxProcs == "" {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	k8Client, err := k8s.GetClient()
	if err != nil {
		level.Error(logger).Log("op", "startup", "k8s", "retrieve client", "error", err)
		os.Exit(1)
	}
	eventRecorder, err := k8s.EventRecorder()
	if err != nil {
		level.Error(logger).Log("op", "startup", "k8s", "retrieve event recorder", "error", err)
		os.Exit(1)
	}

	// register prometheus http handler
	go func() {
		err = registerPromHandler(*host, *port, *enablePprof)
		if err != nil {
			level.Error(logger).Log("op", "startup", "prometheus", "register", "error", err)
		}
	}()

	nLinkMgr := &utils.MyNetlink{}
	err = initInterfaces(logger, ifaces, allowedTcpPorts, allowedUdpPorts, nLinkMgr)
	if err != nil {
		level.Error(logger).Log("op", "startup", "init", "interface", "error", err)
		os.Exit(1)
	}

	ifEventHandler := netlink.LinkEventHandler{Logger: logger}
	ifEventHandler.Start()

	var monitors []*hostap.InterfaceMonitor
	for _, intf := range ifaces {
		level.Info(logger).Log("op", "startup", "monitor start for interface", intf)
		intfMonitor := hostap.NewInterfaceMonitor(logger, intf, func(intfMonitor *hostap.InterfaceMonitor) {
			intfMonitor.Client = k8Client
			intfMonitor.AuthNsName = authObjKey
			intfMonitor.IfEventHandler = ifEventHandler
			intfMonitor.Recorder = eventRecorder
			intfMonitor.LinkMgr = nLinkMgr
		})
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

	err = resetInterfaces(logger, ifaces, nLinkMgr)
	if err != nil {
		level.Error(logger).Log("op", "shutdown", "reset", "interfaces", "error", err)
	}

	level.Info(logger).Log("op", "shutdown", "msg", "done")
}

func initInterfaces(logger log.Logger, interfaces []string, unprotectedTcpPorts, unprotectedUdpPorts []int, nLinkMgr utils.NetlinkManager) error {
	if interfaces == nil {
		return nil
	}
	for _, iface := range interfaces {
		pfvfs, err := trafficcontrol.GetAssociatedInterfaces(iface, nLinkMgr)
		level.Info(logger).Log("op", "initInterfaces", "pfvfs", pfvfs)
		if err != nil {
			return err
		}
		for _, linkName := range pfvfs {
			err = trafficcontrol.InitInterfaceForEAPTraffic(logger, linkName, unprotectedTcpPorts, unprotectedUdpPorts)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func registerPromHandler(host string, port int, enablePprof bool) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	if enablePprof {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}
	server := &http.Server{
		Addr:              net.JoinHostPort(host, fmt.Sprint(port)),
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	return server.ListenAndServe()
}

func resetInterfaces(logger log.Logger, interfaces []string, nLinkMgr utils.NetlinkManager) error {
	if interfaces == nil {
		return nil
	}
	for _, iface := range interfaces {
		pfvfs, err := trafficcontrol.GetAssociatedInterfaces(iface, nLinkMgr)
		if err != nil {
			return err
		}
		for _, linkName := range pfvfs {
			err = trafficcontrol.ResetInterface(logger, linkName)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func parseIntArgs(arg *string) ([]int, error) {
	var argSlice []int
	if arg == nil || *arg == "" {
		return argSlice, nil
	}
	strSlice := parseStringsArgs(arg)
	for _, str := range strSlice {
		port, err := strconv.Atoi(str)
		if err != nil {
			return nil, err
		}
		argSlice = append(argSlice, port)
	}
	return argSlice, nil
}

func parseStringsArgs(arg *string) []string {
	var argSlice []string
	if arg == nil || *arg == "" {
		return argSlice
	}
	argStr := strings.Split(*arg, ",")
	for _, arg := range argStr {
		argSlice = append(argSlice, strings.TrimSpace(arg))
	}
	return argSlice
}
