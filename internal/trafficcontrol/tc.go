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
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/vishvananda/netlink"
)

var (
	sysClassNet = "/sys/class/net/"
	tcpProtoStr = "tcp"
	udpProtoStr = "udp"
)

func AllowTrafficFromMac(ifName string, macAddress string) error {
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	interfaces, err := GetAssociatedInterfaces(ifName)
	if err != nil {
		return err
	}
	for _, iface := range interfaces {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("tc filter replace dev %s ingress pref 9000 protocol all flower src_mac %s action ok", iface, macAddress))
		err := cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func DenyTrafficFromMac(ifName string, macAddress string) error {
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	interfaces, err := GetAssociatedInterfaces(ifName)
	if err != nil {
		return err
	}
	for _, iface := range interfaces {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("tc filter replace dev %s ingress pref 9000 protocol all flower src_mac %s action drop", iface, macAddress))
		err := cmd.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func InitInterfaceForEAPTraffic(logger log.Logger, ifName string, unprotectTcpPorts, unprotectUdpPorts []int) error {
	if err := ResetInterface(logger, ifName); err != nil {
		return err
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf("tc qdisc add dev %s clsact || return $?", ifName))
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("bash", "-c", fmt.Sprintf("tc filter add dev %s ingress pref 10001 protocol all matchall action drop index 101 || return $?", ifName))
	err = cmd.Run()
	if err != nil {
		return err
	}
	if !IsSriovPF(ifName) {
		return nil
	}
	cmd = exec.Command("bash", "-c", fmt.Sprintf("tc filter add dev %s ingress pref 10000 protocol 0x888e matchall action ok index 100 || return $?", ifName))
	err = cmd.Run()
	if err != nil {
		return err
	}
	if len(unprotectTcpPorts) > 0 {
		err = UnprotectPorts(logger, ifName, tcpProtoStr, unprotectTcpPorts)
		if err != nil {
			return err
		}
	}
	if len(unprotectUdpPorts) > 0 {
		err = UnprotectPorts(logger, ifName, udpProtoStr, unprotectUdpPorts)
		if err != nil {
			return err
		}
	}
	unprotectPorts := append(unprotectTcpPorts, unprotectUdpPorts...)
	if len(unprotectPorts) > 0 {
		err = UnprotectIPv6Ports(logger, ifName, unprotectPorts)
		if err != nil {
			return err
		}
	}
	return nil
}

func ResetInterface(logger log.Logger, ifName string) error {
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	cmd := exec.Command("bash", "-c", fmt.Sprintf("tc qdisc del dev %s ingress >/dev/null 2>&1 || true", ifName))
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("bash", "-c", fmt.Sprintf("tc qdisc del dev %s clsact >/dev/null 2>&1 || true", ifName))
	return cmd.Run()
}

func UnprotectPorts(logger log.Logger, ifName string, protocol string, ports []int) error {
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	for _, port := range ports {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("tc filter add dev %s ingress pref 9999 protocol ip u32 match %s dst %s 0xffff action ok index 99", ifName, protocol, strconv.Itoa(port)))
		err := cmd.Run()
		if err != nil {
			level.Error(logger).Log("op", "tc filter add", "ifName", ifName, "protocol", protocol, "port", port, "error", err)
		}
	}
	return nil
}

func UnprotectIPv6Ports(logger log.Logger, ifName string, ports []int) error {
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	for _, port := range ports {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("tc filter add dev %s ingress pref 9999 protocol ipv6 u32 match ip6 dport %s 0xffff action ok index 100", ifName, strconv.Itoa(port)))
		err := cmd.Run()
		if err != nil {
			level.Error(logger).Log("op", "tc filter add", "ifName", ifName, "protocol", "ipv6", "port", port, "error", err)
		}
	}
	return nil
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
		vfNetDirInfo, err := ioutil.ReadDir(vfNetDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read vf net directory %s: %q", vfNetDir, err)
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
