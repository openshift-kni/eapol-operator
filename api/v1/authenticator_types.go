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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AuthenticatorSpec defines the desired state of a single authenticator instance
type AuthenticatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Enabled controls whether this authenticator is enabled or disabled
	// +kubebuilder:default=true
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

// AuthenticatorStatus defines the observed state of Authenticator
type AuthenticatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
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
