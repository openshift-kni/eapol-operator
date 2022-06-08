#!/bin/bash
#
# Initialize the interfaces defined in the HostAPD config file
#

export UNPROTECTED_TCP_PORTS UNPROTECTED_UDP_PORTS

echo "Initializing interfaces $IFACES"
for iface in ${IFACES//,/ }; do
    /bin/hostapd.action $iface '__INIT__'
done
echo "Initialization complete"
