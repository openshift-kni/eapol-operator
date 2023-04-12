#!/bin/bash
#

pkill wpa_supplicant
TEST_IFACE=enp0s8
echo "start wpa client"
echo $TEST_IFACE
wpa_supplicant -D wired -i $TEST_IFACE -c wpa_supplicant.conf
