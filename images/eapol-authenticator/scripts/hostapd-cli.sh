#!/bin/bash
#
# Start the 'hostapd_cli' listener
#

exec /sbin/hostapd_cli -s /var/run/hostapd/ -r -a /bin/hostapd.actions
