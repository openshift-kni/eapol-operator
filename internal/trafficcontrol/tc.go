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
	"os/exec"
	"strconv"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var (
	sysClassNet = "/sys/class/net/"
	tcpProtoStr = "tcp"
	udpProtoStr = "udp"
)

func AllowTrafficFromMac(pf *PFInfo, macAddress string) error {
	// Restore the original vlan and state on the VF when first client
	// gets authenticated.
	if len(pf.AuthenticatedAddrs) == 1 {
		pf.Authenticated = true
		err := pf.ConfigureVlanStateForVFs()
		if err != nil {
			return err
		}
	}
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	interfaces, err := GetAssociatedInterfaces(pf.Name)
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

func DenyTrafficFromMac(pf *PFInfo, macAddress string) error {
	// When no clients authenticated on PF, then move its VFs into
	// deauthenticated state.
	if len(pf.AuthenticatedAddrs) == 0 {
		pf.Authenticated = false
		err := pf.ConfigureVlanStateForVFs()
		if err != nil {
			return err
		}
	}
	if _, err := exec.LookPath("tc"); err != nil {
		return err
	}
	interfaces, err := GetAssociatedInterfaces(pf.Name)
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
