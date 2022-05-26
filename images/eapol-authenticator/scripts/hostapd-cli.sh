#!/bin/bash
#
# Start the 'hostapd_cli' listener
#

if [[ -z $IFACE ]]; then
    echo '$IFACE must be set to the interface to be monitored'
    exit 1
fi
exec /sbin/hostapd_cli -i "$IFACE" -s /var/run/hostapd/ -r -a /bin/hostapd.actions
