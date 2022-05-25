#!/bin/bash
#
# Initialize the interfaces defined in the HostAPD config file
#

set -ex
[[ -e $CONFIG ]]
ifaceline=$(grep '^interface=' "$CONFIG")
IFACES=${ifaceline#interface=}

echo "Initializing interfaces $IFACES"
for iface in ${a//,/ }; do
    /bin/hostapd.action $iface '__INIT__'
done
echo "Initialization complete"
