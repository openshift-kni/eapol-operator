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

package hostap

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	"github.com/go-kit/log/level"
	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
	"github.com/openshift-kni/eapol-operator/internal/trafficcontrol"
	hostapif "github.com/openshift-kni/eapol-operator/pkg/netlink"
	kapi "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	staConnectedEvent     = "AP-STA-CONNECTED"
	eapSuccessEvent       = "CTRL-EVENT-EAP-SUCCESS"
	staDisconnectedEvent  = "AP-STA-DISCONNECTED"
	eapFailureEvent       = "CTRL-EVENT-EAP-FAILURE"
	pingCommand           = "PING"
	attachCommand         = "ATTACH"
	statusCommand         = "STATUS"
	deauthenticateCommand = "DEAUTHENTICATE"
	unixDgramProtocol     = "unixgram"
	hostapdSocketDir      = "/var/run/hostapd/"
	sockReadBufSize       = 4096
)

var (
	statusReply     = "state="
	solicitedEvents = []string{"PONG\n", "OK\n", statusReply}
	requestTimeout  = int(2 * time.Second / time.Microsecond)
)

type Opts func(intfMonitor *InterfaceMonitor)

type InterfaceMonitor struct {
	Logger         log.Logger
	Client         client.Client
	Recorder       record.EventRecorder
	AuthNsName     *types.NamespacedName
	IfName         string
	PfInfo         *trafficcontrol.PFInfo
	IfEventHandler hostapif.LinkEventHandler
	ifEventCh      chan netlink.LinkUpdate
	hostApdConn    net.Conn
	deauthRequests map[string]int64
	addrMutex      sync.Mutex
	stopWg         sync.WaitGroup
	stop           chan interface{}
	operState      netlink.LinkOperState
	ifEAPState     eapolv1.IfState
}

func (m *InterfaceMonitor) StartMonitor() error {
	local, err := ioutil.TempFile(hostapdSocketDir, "hostapd_monitor_client")
	if err != nil {
		return err
	}
	os.Remove(local.Name())
	conn, err := net.DialUnix(unixDgramProtocol, &net.UnixAddr{Name: local.Name(), Net: unixDgramProtocol},
		&net.UnixAddr{Name: filepath.Join(hostapdSocketDir, m.IfName), Net: unixDgramProtocol})
	if err != nil {
		return err
	}
	m.hostApdConn = conn
	m.stop = make(chan interface{})
	m.stopWg.Add(4)
	m.deauthRequests = make(map[string]int64)
	m.ifEventCh = make(chan netlink.LinkUpdate)
	pfInfo, err := trafficcontrol.GetSriovPFInfo(m.IfName)
	if err != nil {
		return err
	}
	err = pfInfo.ConfigureVlanStateForVFs()
	if err != nil {
		return err
	}
	m.PfInfo = pfInfo
	m.IfEventHandler.Subscribe(m.ifEventCh, m.IfName)
	go m.handleHostapdReply()
	go m.handleIfEvents()
	go m.sendKeepAlive()
	go m.handleRequestsTimeout()
	err = m.attachHostapd()
	if err != nil {
		m.StopMonitor()
		return err
	}
	m.writeCommand(statusCommand)
	if err := m.updateInterfaceStatus(); err != nil {
		level.Info(m.Logger).Log("op", "monitor", "error updating interface status", err)
	}
	level.Info(m.Logger).Log("op", "monitor", "interface monitor started", m.IfName)
	return nil
}

func (m *InterfaceMonitor) StopMonitor() {
	if m.PfInfo != nil && !m.PfInfo.Authenticated {
		m.PfInfo.Authenticated = true
		err := m.PfInfo.ConfigureVlanStateForVFs()
		if err != nil {
			level.Error(m.Logger).Log("error resoring vf state and vlan configuration", m.IfName, "error", err)
		}
	}
	close(m.stop)
	m.IfEventHandler.Unsubscribe(m.IfName)
	err := m.hostApdConn.Close()
	if err != nil {
		level.Error(m.Logger).Log("sockread", "error closing connection", m.IfName, err)
	}
	m.stopWg.Wait()
}

func (m *InterfaceMonitor) handleHostapdReply() {
	defer m.stopWg.Done()
	receivedByteArr := make([]byte, sockReadBufSize)
	for {
		select {
		case <-m.stop:
			return
		default:
			m.hostApdConn.SetReadDeadline(time.Now().Add(1 * time.Second))
			size, err := m.hostApdConn.Read(receivedByteArr)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				level.Error(m.Logger).Log("sockread", "error reading from connection", m.IfName, err)
				return
			}
			if size == 0 {
				continue
			}
			eventStr := string(receivedByteArr[:size])
			err = m.handleHostapdEvent(eventStr)
			if err != nil {
				level.Error(m.Logger).Log("hostapd-event", eventStr, "interface", m.IfName, "error", err)
			}
		}
	}
}

func (m *InterfaceMonitor) handleHostapdEvent(eventStr string) error {
	eventStrSlice := strings.Split(eventStr, " ")
	if len(eventStrSlice) < 2 {
		if isSolicitedEvent(eventStr) {
			if strings.Contains(eventStr, statusReply) {
				m.ifEAPState = getIfState(eventStr)
				if err := m.updateInterfaceStatus(); err != nil {
					level.Info(m.Logger).Log("op", "monitor", "error updating interface status", err)
				}
			}
			return nil
		}
		level.Info(m.Logger).Log("hostapd-event", "unhandled event", m.IfName, eventStr)
		return nil
	}
	defer func() {
		if err := m.updateInterfaceStatus(); err != nil {
			level.Info(m.Logger).Log("op", "monitor", "error updating interface status", err)
		}
	}()
	eventKeyStr := eventStrSlice[0][strings.Index(eventStrSlice[0], ">")+1:]
	switch eventKeyStr {
	case staConnectedEvent:
		return m.handleAuthenticateEvent(eventStrSlice[1])
	case eapSuccessEvent:
		m.logEvent(kapi.EventTypeNormal, "authenticated supplicant %s", eventStrSlice[1])
		stats.Authenticated(m.IfName)
	case staDisconnectedEvent:
		m.logEvent(kapi.EventTypeNormal, "deauthenticated supplicant %s", eventStrSlice[1])
		stats.DeAuthenticated(m.IfName)
		return m.handleDeAuthenticateEvent(eventStrSlice[1])
	case eapFailureEvent:
		m.logEvent(kapi.EventTypeWarning, "authentication failure for supplicant %s", eventStrSlice[1])
		stats.AuthFailed(m.IfName)
	default:
		level.Info(m.Logger).Log("hostapd-event", "unhandled event", m.IfName, eventStr)
	}
	return nil
}

func (m *InterfaceMonitor) sendKeepAlive() {
	defer m.stopWg.Done()
	for {
		select {
		case <-m.stop:
			return
		default:
			time.Sleep(1 * time.Second)
			m.hostApdConn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			_, err := m.hostApdConn.Write([]byte(pingCommand))
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
					continue
				}
				level.Error(m.Logger).Log("sockwrite", "error writing ping command to hostapd", m.IfName, err)
				return
			}
		}
	}
}

func (m *InterfaceMonitor) attachHostapd() error {
	m.hostApdConn.SetWriteDeadline(time.Now().Add(1 * time.Second))
	for {
		_, err := m.hostApdConn.Write([]byte(attachCommand))
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Timeout() {
				continue
			}
			level.Error(m.Logger).Log("sockwrite", "error writing attach command to hostapd", m.IfName, err)
			return err
		}
		break
	}
	return nil
}

func (m *InterfaceMonitor) handleIfEvents() {
	defer m.stopWg.Done()
	for {
		select {
		case <-m.stop:
			close(m.ifEventCh)
			return
		case linkUpdateEvent := <-m.ifEventCh:
			if linkUpdateEvent.Link.Attrs().OperState != m.operState {
				m.handlePfEventForOpStateChange(linkUpdateEvent)
			}
			m.handlePfEventForVfVlanChange(linkUpdateEvent)
		}
	}
}

func (m *InterfaceMonitor) handlePfEventForVfVlanChange(linkUpdateEvent netlink.LinkUpdate) {
	m.addrMutex.Lock()
	defer m.addrMutex.Unlock()
	err := m.PfInfo.HandlePfEventForVlanChange(m.Logger)
	if err != nil {
		level.Error(m.Logger).Log("error handling pf event", m.IfName, "event",
			linkUpdateEvent, "error", err)
	}
}

func (m *InterfaceMonitor) handlePfEventForOpStateChange(linkUpdateEvent netlink.LinkUpdate) {
	m.operState = linkUpdateEvent.Link.Attrs().OperState
	level.Info(m.Logger).Log("interface", "event", m.IfName, "op state changed", m.operState)
	m.addrMutex.Lock()
	if m.operState == netlink.OperDown {
		for addr := range m.PfInfo.AuthenticatedAddrs {
			err := m.deauthenticate(addr)
			if err != nil {
				level.Error(m.Logger).Log("interface", "addr", m.IfName, addr, "error deauthenticating addr", err)
			}
			m.deauthRequests[addr] = getCurrentTimestamp()
		}
		m.logEvent(kapi.EventTypeWarning, "interface is down")
	} else if m.operState == netlink.OperUp {
		for addr := range m.PfInfo.AuthenticatedAddrs {
			delete(m.deauthRequests, addr)
		}
		m.logEvent(kapi.EventTypeNormal, "interface is up")
	}
	level.Info(m.Logger).Log("interface", m.IfName, "handleIfEvents", m.deauthRequests)
	m.addrMutex.Unlock()
	m.writeCommand(statusCommand)
}

func (m *InterfaceMonitor) handleRequestsTimeout() {
	defer m.stopWg.Done()
	for {
		select {
		case <-m.stop:
			return
		default:
			now := getCurrentTimestamp()
			m.addrMutex.Lock()
			for addr, reqTimeStamp := range m.deauthRequests {
				if (now - reqTimeStamp) < int64(requestTimeout) {
					// skip remaining requests for addr as they are not timed out as well.
					break
				}
				delete(m.PfInfo.AuthenticatedAddrs, addr)
				err := trafficcontrol.DenyTrafficFromMac(m.PfInfo, addr)
				if err != nil {
					level.Error(m.Logger).Log("interface", "addr", m.IfName, addr, "error applying deny traffic", err)
				}
				delete(m.deauthRequests, addr)
			}
			m.addrMutex.Unlock()
			time.Sleep(1 * time.Second)
		}
	}
}

func (m *InterfaceMonitor) deauthenticate(addr string) error {
	if addr == "" {
		return nil
	}
	_, err := m.hostApdConn.Write([]byte(fmt.Sprintf("%s %s", deauthenticateCommand, addr)))
	return err
}

func (m *InterfaceMonitor) writeCommand(command string) error {
	_, err := m.hostApdConn.Write([]byte(command))
	return err
}

func (m *InterfaceMonitor) handleAuthenticateEvent(addr string) error {
	m.addrMutex.Lock()
	defer m.addrMutex.Unlock()
	m.PfInfo.AuthenticatedAddrs[addr] = nil
	delete(m.deauthRequests, addr)
	return trafficcontrol.AllowTrafficFromMac(m.PfInfo, addr)
}

func (m *InterfaceMonitor) handleDeAuthenticateEvent(addr string) error {
	m.addrMutex.Lock()
	defer m.addrMutex.Unlock()
	delete(m.PfInfo.AuthenticatedAddrs, addr)
	delete(m.deauthRequests, addr)
	return trafficcontrol.DenyTrafficFromMac(m.PfInfo, addr)
}

func (m *InterfaceMonitor) updateInterfaceStatus() error {
	m.addrMutex.Lock()
	defer m.addrMutex.Unlock()
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		authObj, err := m.getAuthObject()
		if err != nil {
			return err
		}
		var ifStatus *eapolv1.Interface
		for i, iface := range authObj.Status.Interfaces {
			if iface.Name == m.IfName {
				ifStatus = authObj.Status.Interfaces[i]
				break
			}
		}
		if ifStatus == nil {
			ifStatus = &eapolv1.Interface{Name: m.IfName}
			authObj.Status.Interfaces = append(authObj.Status.Interfaces, ifStatus)
		}
		ifStatus.State = m.ifEAPState
		ifStatus.AuthenticatedClients = []string{}
		for sta := range m.PfInfo.AuthenticatedAddrs {
			ifStatus.AuthenticatedClients = append(ifStatus.AuthenticatedClients, sta)
		}
		return m.Client.Status().Update(context.Background(), authObj, &client.SubResourceUpdateOptions{})
	})
}

func (m *InterfaceMonitor) logEvent(eventType, messageFmt string, args ...interface{}) {
	authObj, err := m.getAuthObject()
	if err != nil {
		level.Error(m.Logger).Log("record-event", "error recording event", m.IfName, err)
	}
	m.Recorder.Eventf(authObj, eventType, m.IfName, messageFmt, args...)
}

func (m *InterfaceMonitor) getAuthObject() (*eapolv1.Authenticator, error) {
	authObj := &eapolv1.Authenticator{}
	err := m.Client.Get(context.Background(), *m.AuthNsName, authObj)
	if err != nil {
		return nil, err
	}
	return authObj, nil
}

func NewInterfaceMonitor(logger log.Logger, ifName string, opts ...Opts) *InterfaceMonitor {
	intfMonitor := &InterfaceMonitor{Logger: logger, IfName: ifName}
	for _, opt := range opts {
		opt(intfMonitor)
	}
	return intfMonitor
}

func isSolicitedEvent(eventStr string) bool {
	for _, solicit := range solicitedEvents {
		if strings.Contains(eventStr, solicit) {
			return true
		}
	}
	return false
}

func getIfState(eventStr string) eapolv1.IfState {
	states := strings.Split(eventStr, "\n")
	statuses := map[string]string{}
	for _, state := range states {
		part := strings.Split(state, "=")
		if len(part) == 1 {
			statuses[part[0]] = ""
		}
		if len(part) == 2 {
			statuses[part[0]] = part[1]
		}
	}
	state := statuses["state"]
	switch state {
	case "UNINITIALIZED":
		return eapolv1.IfStateUninitialized
	case "DISABLED":
		return eapolv1.IfStateDisabled
	case "COUNTRY_UPDATE":
		return eapolv1.IfStateCountryUpdate
	case "ACS":
		return eapolv1.IfStateAcs
	case "HT_SCAN":
		return eapolv1.IfStateHtScan
	case "DFS":
		return eapolv1.IfStateDfs
	case "ENABLED":
		return eapolv1.IfStateEnabled
	default:
		return eapolv1.IfStateUnknown
	}
}

func getCurrentTimestamp() int64 {
	return time.Now().UnixMicro()
}
