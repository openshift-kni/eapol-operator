/*
Copyright 2022.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IfState string

var (
	IfStateUninitialized IfState = "Uninitialized"
	IfStateDisabled      IfState = "Disabled"
	IfStateCountryUpdate IfState = "CountryUpdate"
	IfStateAcs           IfState = "ACS"
	IfStateHtScan        IfState = "HT Scan"
	IfStateDfs           IfState = "DFS"
	IfStateEnabled       IfState = "Enabled"
	IfStateUnknown       IfState = "Unknown"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AuthenticatorSpec defines the desired state of a single authenticator instance
type AuthenticatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Enabled controls whether this authenticator is enabled or disabled
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled"`

	// Interfaces is the list of interfaces to protect under this authenticator instance
	Interfaces []string `json:"interfaces"`

	// Authentication configures back-end authentication for this authenticator
	Authentication Auth `json:"authentication"`

	// Configuration contains various low-level EAP tunable values
	// +optional
	Configuration *Config `json:"configuration,omitempty"`

	// Image optionally overrides the default eapol-authenticator container image
	// +optional
	Image string `json:"image,omitempty"`

	// NodeSelector limits the nodes that the authenticator can run on
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// TrafficControl configures the traffic allowed in and out when
	// authenticated and not authenticated.  If unset, the default is to
	// disallow all traffic until authenticated, and then allow all traffic.
	// +optional
	TrafficControl *TrafficControl `json:"trafficControl,omitempty"`
}

// Auth represents back-end authentication configuration
type Auth struct {
	// Local configures the local internal authentication server
	// +optional
	Local *Local `json:"local,omitempty"`

	// Radius is the external RADIUS server configuration to use for authentication
	// +optional
	Radius *Radius `json:"radius,omitempty"`
}

// Local represents a local EAP authentication configuration
type Local struct {
	// UserFileSecret configures the local authentication user file based on a secret contents.
	// If the key is not specified, it is assumed to be "hostapd.eap_user"
	// +optional
	UserFileSecret *SecretKeyRef `json:"userFileSecret,omitempty"`
	// CaCertSecret secret reference containing certificate authority for hostapd daemon.
	// If the key is not specified, it is assumed to be "1x-ca.pem"
	// +optional
	CaCertSecret *SecretKeyRef `json:"caCertSecret,omitempty"`
	// ServerCertSecret secret reference containing server certificate for hostapd daemon.
	// If the key is not specified, it is assumed to be "1x-hostapd.example.com.pem"
	// +optional
	ServerCertSecret *SecretKeyRef `json:"serverCertSecret,omitempty"`
	// PrivateKeySecret secret reference containing private key for hostapd daemon server certificate.
	// If the key is not specified, it is assumed to be "1x-hostapd.example.com.key"
	// +optional
	PrivateKeySecret *SecretKeyRef `json:"privateKeySecret,omitempty"`
	// PrivateKeyPassphrase containing passphrase for the private key.
	// +optional
	PrivateKeyPassphrase string `json:"privateKeyPassphrase,omitempty"`
	// RadiusClientSecret secret reference containing client information for local radius server.
	// If the key is not specified, it is assumed to be "hostapd.radius_clients"
	// +optional
	RadiusClientSecret *SecretKeyRef `json:"radiusClientFileSecret,omitempty"`
	// AuthPort UDP listening port Local Radius authentication server.
	// +kubebuilder:default=1812
	// +optional
	AuthPort int `json:"authPort"`
}

// Radius represents a RADIUS server configuration
type Radius struct {
	// AuthServer is the IP address or hostname of the RADIUS authentication server
	AuthServer string `json:"authServer"`

	// AuthPort is the TCP Port of the RADIUS authentication server
	AuthPort int `json:"authPort"`

	// AuthSecret is the name of the Secret that contains the RADIUS authentication server shared secret
	AuthSecret string `json:"authSecret"`
}

type SecretKeyRef struct {
	// Name is the name of the secret to reference
	Name string `json:"name"`

	// Key is the key in the secret to refer to
	// +optional
	Key string `json:"key,omitempty"`
}

// Config represents miscelaneous 802.1x and EAP tunable values
type Config struct {
	// EapReauthPeriod is the EAP reauthentication period in seconds (default: 3600 seconds; 0 = disable)
	// +kubebuilder:default=3600
	EapReauthPeriod int `json:"eapReauthPeriod"`
}

// TrafficControl represents the traffic control for hostapd.
type TrafficControl struct {
	// UnprotectedPorts is a list of ingress destination ports to allow even for unathenticated interfaces
	// +optional
	UnprotectedPorts *Ports `json:"unprotectedPorts,omitempty"`
}

// Port represents a single IP port
type Ports struct {
	// Tcp is a list of tcp ports
	// +optional
	Tcp []int `json:"tcp,omitempty"`

	// Udp is a lits of udp ports
	// +optional
	Udp []int `json:"udp,omitempty"`
}

// AuthenticatorStatus defines the observed state of Authenticator
type AuthenticatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Interfaces is the list of interface status
	// +optional
	Interfaces []*Interface `json:"interfaces,omitempty"`
}

type Interface struct {
	// Name is the name of the interface
	Name string `json:"name"`
	// State is the state of the interface. The possible states are Uninitialized,
	// Disabled, CountryUpdate, ACS, HT Scan, DFS, Enabled or Unknown.
	State IfState `json:"status"`
	// AuthenticatedClients is the list of authenticated stations on the interface
	// +optional
	AuthenticatedClients []string `json:"authenticatedClients"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Authenticator is the Schema for the authenticators API
type Authenticator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthenticatorSpec   `json:"spec,omitempty"`
	Status AuthenticatorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AuthenticatorList contains a list of Authenticator
type AuthenticatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Authenticator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Authenticator{}, &AuthenticatorList{})
}
