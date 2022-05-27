#!/bin/bash
#
# Initialize the interfaces defined in the HostAPD config file
#

echo "Initializing interfaces $IFACES"
for iface in ${IFACES//,/ }; do
    /bin/hostapd.action $iface '__INIT__'
done
echo "Initialization complete"
