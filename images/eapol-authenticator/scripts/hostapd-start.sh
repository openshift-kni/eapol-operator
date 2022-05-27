#!/bin/bash
#
# Launch hostapd with the same config for each interface
#

# hostapd can take a comma-delimited list of interfaces to the '-i' argument,
# but must have one config file provided for each interface.  So we just repeat
# the same config for each interface.
CONFIGS=()
for iface in ${IFACES//,/ }; do
    CONFIGS+=($CONFIG)
done
exec /sbin/hostapd -i "$IFACES" "${CONFIGS[@]}"
