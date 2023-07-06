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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/vishvananda/netlink"
)

const (
	ReservedVlan = 4095
)

type PFInfo struct {
	Name               string
	Authenticated      bool
	AuthenticatedAddrs map[string]interface{}
	VFs                map[int]*VFInfo
}

type VFInfo struct {
	Parent *PFInfo
	Index  int
	Vlan   int
}

func (pf *PFInfo) HandlePfEventForVlanChange(logger log.Logger) error {
	pfLink, err := netlink.LinkByName(pf.Name)
	if err != nil {
		return err
	}
	for _, vf := range pfLink.Attrs().Vfs {
		if vfInfo, ok := pf.VFs[vf.ID]; ok && vf.Vlan != ReservedVlan &&
			vfInfo.Vlan != vf.Vlan {
			vfInfo.Vlan = vf.Vlan
			level.Info(logger).Log("interface", "event", pf.Name, "vlan changed", vfInfo)
			err := vfInfo.ConfigureVlanState()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (pf *PFInfo) ConfigureVlanStateForVFs() error {
	for _, vfInfo := range pf.VFs {
		err := vfInfo.ConfigureVlanState()
		if err != nil {
			return err
		}
	}
	return nil
}

func (vf *VFInfo) ConfigureVlanState() error {
	pfLink, err := netlink.LinkByName(vf.Parent.Name)
	if err != nil {
		return err
	}
	var (
		vlan  int
		state uint32
	)
	if !vf.Parent.Authenticated {
		// Use reserved vlan value 4095 when PF is in unauthenticated state.
		vlan = ReservedVlan
		state = netlink.VF_LINK_STATE_DISABLE
	} else {
		state = netlink.VF_LINK_STATE_AUTO
		vlan = vf.Vlan
	}
	err = netlink.LinkSetVfVlan(pfLink, vf.Index, vlan)
	if err != nil {
		return err
	}
	return netlink.LinkSetVfState(pfLink, vf.Index, state)
}

func GetSriovPFInfo(ifName string) (*PFInfo, error) {
	pfLink, err := netlink.LinkByName(ifName)
	if err != nil {
		return nil, err
	}
	pf := &PFInfo{Name: ifName, Authenticated: false, VFs: map[int]*VFInfo{},
		AuthenticatedAddrs: make(map[string]interface{})}
	for _, vf := range pfLink.Attrs().Vfs {
		pf.VFs[vf.ID] = &VFInfo{Parent: pf, Index: vf.ID, Vlan: vf.Vlan}
	}
	return pf, nil
}

func GetSriovVFs(ifName string) ([]string, error) {
	vfNames := []string{}
	if !IsSriovPF(ifName) {
		return nil, fmt.Errorf("interface %s is not a sriov pf type", ifName)
	}
	vfFnsDir := filepath.Join(sysClassNet, ifName, "device", "virtfn*")
	vfsDir, err := filepath.Glob(vfFnsDir)
	if err != nil {
		return nil, err
	}
	for _, vfDir := range vfsDir {
		vfNetDir := filepath.Join(vfDir, "net")
		// Skip the VF if it is already bound with dpdk driver.
		if !dirExists(vfNetDir) {
			continue
		}
		vfNetDirInfo, err := ioutil.ReadDir(vfNetDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read vf net directory %s: %q", vfNetDir, err)
		}
		// Skip the VF if it is alreay moved into another network namespace.
		if len(vfNetDirInfo) == 0 {
			continue
		}
		vfIfName := vfNetDirInfo[0].Name()
		vfLink, err := netlink.LinkByName(vfIfName)
		if err != nil {
			return nil, fmt.Errorf("failed to get netlink %s: %q", vfIfName, err)
		}
		vfNames = append(vfNames, vfLink.Attrs().Name)
	}
	return vfNames, nil
}

func IsSriovPF(ifName string) bool {
	ifPfDir := filepath.Join(sysClassNet, ifName, "device", "sriov_numvfs")
	if _, err := os.Stat(ifPfDir); err != nil {
		return false
	}
	return true
}

func GetAssociatedInterfaces(ifName string) ([]string, error) {
	interfaces := []string{ifName}
	if IsSriovPF(ifName) {
		vfs, err := GetSriovVFs(ifName)
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, vfs...)
	}
	return interfaces, nil
}

func dirExists(dirname string) bool {
	info, err := os.Stat(dirname)
	return err == nil && info.IsDir()
}
