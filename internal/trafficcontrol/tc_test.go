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

package trafficcontrol

import (
	"net"

	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
)

var _ = Describe("tc", func() {
	var (
		//logger log.Logger
		t GinkgoTInterface
	)
	BeforeEach(func() {
		var err error
		//logger, err = logging.Init("info")
		Expect(err).NotTo(HaveOccurred())
		t = GinkgoT()
	})

	Context("Validating allow and deny mac functions", func() {
		BeforeEach(func() {
			t = GinkgoT()
		})
		It("allow traffic on a mac address", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e1")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			pfInfo := &PFInfo{Name: pfName, Authenticated: true, AuthenticatedAddrs: map[string]interface{}{"6e:16:06:0e:b7:e2": nil},
				VFs: map[int]*VFInfo{0: {Index: 0, Vlan: 200,
					Parent: &PFInfo{Name: pfName, Authenticated: true, nLinkMgr: mocked}}},
				nLinkMgr: mocked}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []netlink.VfInfo{{ID: 0, Vlan: 200}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, 200).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, netlink.VF_LINK_STATE_AUTO).Return(nil)
			err = AllowTrafficFromMac(pfInfo, "6e:16:06:0e:b7:e2", mocked)
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}
			mocked.AssertExpectations(t)
		})

		It("deny traffic on a mac address", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e1")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			pfInfo := &PFInfo{Name: pfName, Authenticated: true,
				VFs: map[int]*VFInfo{0: {Index: 0, Vlan: 200,
					Parent: &PFInfo{Name: pfName, Authenticated: false, nLinkMgr: mocked}}},
				nLinkMgr: mocked}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []netlink.VfInfo{{ID: 0, Vlan: 200}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, netlink.VF_LINK_STATE_DISABLE).Return(nil)
			err = DenyTrafficFromMac(pfInfo, "6e:16:06:0e:b7:e2", mocked)
			if err != nil {
				// Ignore if the error occurred while running tc command.
				Expect(err.Error()).To(Equal("exit status 1"))
			}
			mocked.AssertExpectations(t)
		})
	})
})
