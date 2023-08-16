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
	"net"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eapol-operator/internal/logging"
	"github.com/openshift-kni/eapol-operator/internal/trafficcontrol"
	"github.com/openshift-kni/eapol-operator/pkg/netlink"
	"github.com/stretchr/testify/mock"
	vnetlink "github.com/vishvananda/netlink"
)

var (
	pfName = "enp175s0f1"
)

var _ = Describe("Hostap", func() {
	var (
		logger log.Logger
	)
	BeforeEach(func() {
		var err error
		logger, err = logging.Init("info")
		Expect(err).NotTo(HaveOccurred())
		Expect(logger).NotTo(BeNil())
	})
	Context("Test hostapd monitor", func() {
		var (
			conn     *net.UnixConn
			err      error
			sockFile string
		)
		BeforeEach(func() {
			hostapdSocketDir, err = os.MkdirTemp("/tmp", "hostapd-test-")
			Expect(err).NotTo(HaveOccurred())
			sockFile = fmt.Sprintf("%s/%s", hostapdSocketDir, pfName)
			conn, err = net.ListenUnixgram(unixDgramProtocol, &net.UnixAddr{Name: sockFile, Net: unixDgramProtocol})
			Expect(err).NotTo(HaveOccurred())
			Expect(conn).NotTo(BeNil())
		})
		AfterEach(func() {
			conn.Close()
			os.Remove(hostapdSocketDir)
		})
		It("Hostap monitor start and stop", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: vnetlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []vnetlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			ifEventHandler := netlink.LinkEventHandler{Logger: logger}
			ifEventHandler.Start()
			intfMonitor := NewInterfaceMonitor(logger, pfName, func(intfMonitor *InterfaceMonitor) {
				intfMonitor.IfEventHandler = ifEventHandler
				intfMonitor.LinkMgr = mocked
			})
			err = intfMonitor.StartMonitor()
			Expect(err).NotTo(HaveOccurred())
			mocked.On("LinkSetVfVlan", fakeLink, 0, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_AUTO).Return(nil)
			ch := make(chan struct{})
			go func() {
				intfMonitor.StopMonitor()
				ifEventHandler.StopHandler()
				close(ch)
			}()
			Eventually(func() bool {
				select {
				case <-ch:
					return true
				default:
					return false
				}
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
		})

		It("Validate Hostap authenticate and deauthenticate event", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: vnetlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []vnetlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			ifEventHandler := netlink.LinkEventHandler{Logger: logger}
			ifEventHandler.Start()
			intfMonitor := NewInterfaceMonitor(logger, pfName, func(intfMonitor *InterfaceMonitor) {
				intfMonitor.IfEventHandler = ifEventHandler
				intfMonitor.LinkMgr = mocked
			})
			err = intfMonitor.StartMonitor()
			Expect(err).NotTo(HaveOccurred())
			// Authenticate mac address 6e:16:06:0e:b7:e2.
			mocked.On("LinkSetVfVlan", fakeLink, 0, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_AUTO).Return(nil)
			err = intfMonitor.handleAuthenticateEvent("6e:16:06:0e:b7:e2")
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}
			// Deauthenticate mac address 6e:16:06:0e:b7:e2.
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			err = intfMonitor.handleDeAuthenticateEvent("6e:16:06:0e:b7:e2")
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}
			mocked.On("LinkSetVfVlan", fakeLink, 0, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_AUTO).Return(nil)
			ch := make(chan struct{})
			go func() {
				intfMonitor.StopMonitor()
				ifEventHandler.StopHandler()
				close(ch)
			}()
			Eventually(func() bool {
				select {
				case <-ch:
					return true
				default:
					return false
				}
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
		})

		It("Validate interface events while hostap monitor running", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: vnetlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []vnetlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			ifEventHandler := netlink.LinkEventHandler{Logger: logger}
			ifEventHandler.Start()
			intfMonitor := NewInterfaceMonitor(logger, pfName, func(intfMonitor *InterfaceMonitor) {
				intfMonitor.IfEventHandler = ifEventHandler
				intfMonitor.LinkMgr = mocked
			})
			err = intfMonitor.StartMonitor()
			Expect(err).NotTo(HaveOccurred())

			// Validate vlan change event when PF is in unauthenticated state.
			fakeLink.Vfs[0].Vlan = 200
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			intfMonitor.handlePfEventForVfVlanChange(vnetlink.LinkUpdate{Link: fakeLink})

			// Validate vlan change event when PF is in authenticated state.
			mocked.On("LinkSetVfVlan", fakeLink, 0, 200).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_AUTO).Return(nil)
			err = intfMonitor.handleAuthenticateEvent("6e:16:06:0e:b7:e2")
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}
			fakeLink.Vfs[0].Vlan = 300
			mocked.On("LinkSetVfVlan", fakeLink, 0, 300).Return(nil)
			intfMonitor.handlePfEventForVfVlanChange(vnetlink.LinkUpdate{Link: fakeLink})

			fakeLink.LinkAttrs.OperState = vnetlink.OperDown
			Expect(len(intfMonitor.deauthRequests)).To(Equal(0))
			intfMonitor.handlePfEventForOpStateChange(vnetlink.LinkUpdate{Link: fakeLink})
			Expect(len(intfMonitor.deauthRequests)).To(Equal(1))
			fakeLink.LinkAttrs.OperState = vnetlink.OperUp
			intfMonitor.handlePfEventForOpStateChange(vnetlink.LinkUpdate{Link: fakeLink})
			Expect(len(intfMonitor.deauthRequests)).To(Equal(0))

			ch := make(chan struct{})
			go func() {
				intfMonitor.StopMonitor()
				ifEventHandler.StopHandler()
				close(ch)
			}()
			Eventually(func() bool {
				select {
				case <-ch:
					return true
				default:
					return false
				}
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
		})

		It("Validate hostap events while hostap monitor running", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: vnetlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []vnetlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			ifEventHandler := netlink.LinkEventHandler{Logger: logger}
			ifEventHandler.Start()
			intfMonitor := NewInterfaceMonitor(logger, pfName, func(intfMonitor *InterfaceMonitor) {
				intfMonitor.IfEventHandler = ifEventHandler
				intfMonitor.LinkMgr = mocked
			})
			err = intfMonitor.StartMonitor()
			Expect(err).NotTo(HaveOccurred())

			// Test solicited events.
			err = intfMonitor.handleHostapdEvent("PONG\n")
			Expect(err).NotTo(HaveOccurred())
			err = intfMonitor.handleHostapdEvent("OK\n")
			Expect(err).NotTo(HaveOccurred())
			// Test an unknown event.
			err = intfMonitor.handleHostapdEvent("unknown-event")
			Expect(err).NotTo(HaveOccurred())

			// Test ap sta connected event.
			mocked.On("LinkSetVfVlan", fakeLink, 0, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_AUTO).Return(nil)
			err = intfMonitor.handleHostapdEvent("AP-STA-CONNECTED 6e:16:06:0e:b7:e2")
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}

			// Test ap disconnected event.
			mocked.On("LinkSetVfVlan", fakeLink, 0, trafficcontrol.ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_DISABLE).Return(nil)
			err = intfMonitor.handleHostapdEvent("AP-STA-DISCONNECTED 6e:16:06:0e:b7:e2")
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}

			mocked.On("LinkSetVfVlan", fakeLink, 0, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, vnetlink.VF_LINK_STATE_AUTO).Return(nil)
			ch := make(chan struct{})
			go func() {
				intfMonitor.StopMonitor()
				ifEventHandler.StopHandler()
				close(ch)
			}()
			Eventually(func() bool {
				select {
				case <-ch:
					return true
				default:
					return false
				}
			}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
		})
	})
})
