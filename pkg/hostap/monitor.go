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

	"github.com/go-kit/log/level"
	"github.com/openshift-kni/eapol-operator/internal/trafficcontrol"
	hostapif "github.com/openshift-kni/eapol-operator/pkg/netlink"
)

const (
	staConnectedEvent     = "AP-STA-CONNECTED"
	eapSuccessEvent       = "CTRL-EVENT-EAP-SUCCESS"
	staDisconnectedEvent  = "AP-STA-DISCONNECTED"
	eapFailureEvent       = "CTRL-EVENT-EAP-FAILURE"
	pingCommand           = "PING"
	attachCommand         = "ATTACH"
	deauthenticateCommand = "DEAUTHENTICATE"
	unixDgramProtocol     = "unixgram"
	hostapdSocketDir      = "/var/run/hostapd/"
	sockReadBufSize       = 4096
)

var (
	solicitedEvents = []string{"PONG\n", "OK\n"}
	requestTimeout  = int(2 * time.Second / time.Microsecond)
)

type InterfaceMonitor struct {
	Logger             log.Logger
	IfName             string
	IfEventHandler     hostapif.LinkEventHandler
	ifEventCh          chan netlink.LinkUpdate
	hostApdConn        net.Conn
	authenticatedAddrs map[string]interface{}
	deauthRequests     map[string]int64
	addrMutex          sync.Mutex
	stopWg             sync.WaitGroup
	stop               chan interface{}
	operState          netlink.LinkOperState
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
	m.authenticatedAddrs = make(map[string]interface{})
	m.stop = make(chan interface{})
	m.stopWg.Add(4)
	m.deauthRequests = make(map[string]int64)
	m.ifEventCh = make(chan netlink.LinkUpdate)
	m.IfEventHandler.Subscribe(m.IfName, m.ifEventCh)
	go m.handleHostapdReply()
	go m.handleIfEvents()
	go m.sendKeepAlive()
	go m.handleRequestsTimeout()
	err = m.attachHostapd()
	if err != nil {
		m.StopMonitor()
		return err
	}
	level.Info(m.Logger).Log("op", "monitor", "interface monitor started", m.IfName)
	return nil
}

func (m *InterfaceMonitor) StopMonitor() {
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
			return nil
		}
		level.Info(m.Logger).Log("hostapd-event", "unhandled event", m.IfName, eventStr)
		return nil
	}
	eventKeyStr := eventStrSlice[0][strings.Index(eventStrSlice[0], ">")+1:]
	switch eventKeyStr {
	case staConnectedEvent, eapSuccessEvent:
		return m.handleAuthenticateEvent(eventStrSlice[1])
	case staDisconnectedEvent, eapFailureEvent:
		return m.handleDeAuthenticateEvent(eventStrSlice[1])
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
			newOpState := linkUpdateEvent.Link.Attrs().OperState
			if newOpState == m.operState {
				level.Info(m.Logger).Log("interface", "event", m.IfName, "op state not changed", m.operState)
				continue
			}
			m.operState = newOpState
			level.Info(m.Logger).Log("interface", "event", m.IfName, "op state changed", m.operState)
			m.addrMutex.Lock()
			if newOpState == netlink.OperDown {
				for addr := range m.authenticatedAddrs {
					err := m.deauthenticate(addr)
					if err != nil {
						level.Error(m.Logger).Log("interface", "addr", m.IfName, addr, "error deauthenticating addr", err)
					}
					m.deauthRequests[addr] = getCurrentTimestamp()
				}
			} else if newOpState == netlink.OperUp {
				for addr := range m.authenticatedAddrs {
					delete(m.deauthRequests, addr)
				}
			}
			level.Info(m.Logger).Log("interface", m.IfName, "handleIfEvents", m.deauthRequests)
			m.addrMutex.Unlock()
		}
	}
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
				delete(m.authenticatedAddrs, addr)
				err := trafficcontrol.DenyTrafficFromMac(m.IfName, addr)
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

func (m *InterfaceMonitor) handleAuthenticateEvent(addr string) error {
	m.addrMutex.Lock()
	defer m.addrMutex.Unlock()
	m.authenticatedAddrs[addr] = nil
	delete(m.deauthRequests, addr)
	return trafficcontrol.AllowTrafficFromMac(m.IfName, addr)
}

func (m *InterfaceMonitor) handleDeAuthenticateEvent(addr string) error {
	m.addrMutex.Lock()
	defer m.addrMutex.Unlock()
	delete(m.authenticatedAddrs, addr)
	delete(m.deauthRequests, addr)
	return trafficcontrol.DenyTrafficFromMac(m.IfName, addr)
}

func isSolicitedEvent(eventStr string) bool {
	for _, solicit := range solicitedEvents {
		if eventStr == solicit {
			return true
		}
	}
	return false
}

func getCurrentTimestamp() int64 {
	return time.Now().UnixMicro()
}
