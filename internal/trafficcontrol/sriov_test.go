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

	"github.com/go-kit/log"
	"github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils"
	mocks_utils "github.com/k8snetworkplumbingwg/sriov-cni/pkg/utils/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eapol-operator/internal/logging"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
)

var pfName = "enp175s0f1"

var _ = Describe("Sriov", func() {
	var (
		logger log.Logger
		t      GinkgoTInterface
	)
	BeforeEach(func() {
		var err error
		logger, err = logging.Init("info")
		Expect(err).NotTo(HaveOccurred())
		t = GinkgoT()
	})
	Context("Checking HandlePfEventForVlanChange function", func() {
		BeforeEach(func() {
			t = GinkgoT()
		})
		It("Handle Vlan Change on an unauthenticated PF", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			pfInfo := PFInfo{Name: pfName, Authenticated: false,
				VFs: map[int]*VFInfo{0: {Index: 0, Vlan: 200,
					Parent: &PFInfo{Name: pfName, nLinkMgr: mocked}}},
				nLinkMgr: mocked}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []netlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, ReservedVlan).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, netlink.VF_LINK_STATE_DISABLE).Return(nil)
			err = pfInfo.HandlePfEventForVlanChange(logger)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})

		It("Handle Vlan Change on an authenticated PF", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			pfInfo := PFInfo{Name: pfName, Authenticated: true,
				VFs: map[int]*VFInfo{0: {Index: 0, Vlan: 200,
					Parent: &PFInfo{Name: pfName, Authenticated: true, nLinkMgr: mocked}}},
				nLinkMgr: mocked}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []netlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 0, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 0, netlink.VF_LINK_STATE_AUTO).Return(nil)
			err = pfInfo.HandlePfEventForVlanChange(logger)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})

		It("Handle Vlan Change on an authenticated PF configured with multiple VFs", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			pfInfo := PFInfo{Name: pfName, Authenticated: true,
				VFs: map[int]*VFInfo{0: {Index: 0, Vlan: 200,
					Parent: &PFInfo{Name: pfName, Authenticated: true, nLinkMgr: mocked}},
					1: {Index: 1, Vlan: 200,
						Parent: &PFInfo{Name: pfName, Authenticated: true, nLinkMgr: mocked}},
					2: {Index: 2, Vlan: 200,
						Parent: &PFInfo{Name: pfName, Authenticated: true, nLinkMgr: mocked}}},
				nLinkMgr: mocked}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []netlink.VfInfo{{ID: 0, Vlan: 200}, {ID: 1, Vlan: 200}, {ID: 2, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			mocked.On("LinkSetVfVlan", fakeLink, 2, 100).Return(nil)
			mocked.On("LinkSetVfState", fakeLink, 2, netlink.VF_LINK_STATE_AUTO).Return(nil)
			err = pfInfo.HandlePfEventForVlanChange(logger)
			Expect(err).NotTo(HaveOccurred())
			mocked.AssertExpectations(t)
		})
	})

	Context("Validate Sriov PF/VF specific functions", func() {
		BeforeEach(func() {
			t = GinkgoT()
		})
		It("Retrieval of PFInfo using GetSriovPFInfo function", func() {
			fakeMac, err := net.ParseMAC("6e:16:06:0e:b7:e9")
			Expect(err).NotTo(HaveOccurred())
			mocked := &mocks_utils.NetlinkManager{}
			fakeLink := &utils.FakeLink{LinkAttrs: netlink.LinkAttrs{
				Index:        1000,
				Name:         pfName,
				HardwareAddr: fakeMac,
				Vfs:          []netlink.VfInfo{{ID: 0, Vlan: 100}},
			}}
			mocked.On("LinkByName", mock.AnythingOfType("string")).Return(fakeLink, nil)
			pfInfo, err := GetSriovPFInfo(pfName, mocked)
			Expect(err).NotTo(HaveOccurred())
			Expect(pfInfo).NotTo(BeNil())
			Expect(pfInfo.Name).To(Equal(pfName))
			Expect(len(pfInfo.VFs)).To(Equal(1))
			_, err = GetSriovVFs(pfName, mocked)
			Expect(err).To(HaveOccurred())
			ifs, err := GetAssociatedInterfaces(pfName, mocked)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(ifs)).To(Equal(1))
			Expect(ifs[0]).To(Equal(pfName))
		})
	})
})
