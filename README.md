# eapol-operator

An 802.1x authentication operator for Kubernetes

This provides the "authenticator" side of 802.1x network security on secondary
interfaces on a Kubernetes node.  The goal is to allow 802.1x authentication to
occur without and intervening switch, when other network devices are plugged in
to a Kubernetes node directly, such as in the O-RAN LLS-C1 configuration.

## Installation

The OLM bundle is published at
[quay.io/openshift-kni/eapol-operator-bundle](https://eapol-operator-bundle)
and should be installable by pointing OLM at the latest bundle image there.

This operator was built with the [operator
SDk](https://sdk.operatorframework.io/), and the `operator-sdk` tool can
install and run this operator from the source tree.

## Usage

This operator provides a single `Authenticator` CRD that represents a group of
secondary interfaces that should be configured to be 802.1x authenticators.

If you require a different configuration for a different set of interfaces, you
can create multiple CRDs, one for each set of interfaces that share a common
configuration.

Example CRD:

```yaml
apiVersion: eapol.eapol.openshift.io/v1
kind: Authenticator
metadata:
  name: authenticator-sample
spec:
  enabled: true
  interfaces:
    - ens3f0
    - ens3f1
  authentication:
    radius:
      authServer: "authentication-server.external.example.com"
      authSecret: "radius-authsecret"
      authPort: 1812
  configuration:
    eapReauthPeriod: 3600
  trafficControl:
    unprotectedPorts:
      udp:
        - 319
        - 320
```

The operator will report status in the same CRD used for configuration:

```yaml
status:
  interfaces:
    - name: ens3f0
      status: Enabled
      authenticatedClients:
        - 00:00:00:00:00:01
    - name: ens3f1
      status: Disabled
      authenticatedClients: []
```

The 'authenticatedClients' status lists the MAC addresses of any clients
authenticated on the given interface.

## Architecture

The EAPOL-operator starts one daemonset for each configuration (and each
configuration may refer to multiple physical ports). The pod consists of two
containers, one which runs the `hostapd` utility that performs the
authenticator function of the 802.1x protocol, and a `monitor` application
which configures traffic control based on the authentication state.

For SR-IOV interfaces, this operator implements port-based control, and allows
traffic to all VFs once an authentication occurs on the PF.

MACSEC support is not currently implemented.
