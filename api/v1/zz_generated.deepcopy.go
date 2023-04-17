//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Auth) DeepCopyInto(out *Auth) {
	*out = *in
	if in.Local != nil {
		in, out := &in.Local, &out.Local
		*out = new(Local)
		(*in).DeepCopyInto(*out)
	}
	if in.Radius != nil {
		in, out := &in.Radius, &out.Radius
		*out = new(Radius)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Auth.
func (in *Auth) DeepCopy() *Auth {
	if in == nil {
		return nil
	}
	out := new(Auth)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Authenticator) DeepCopyInto(out *Authenticator) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Authenticator.
func (in *Authenticator) DeepCopy() *Authenticator {
	if in == nil {
		return nil
	}
	out := new(Authenticator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Authenticator) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthenticatorList) DeepCopyInto(out *AuthenticatorList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Authenticator, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthenticatorList.
func (in *AuthenticatorList) DeepCopy() *AuthenticatorList {
	if in == nil {
		return nil
	}
	out := new(AuthenticatorList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AuthenticatorList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthenticatorSpec) DeepCopyInto(out *AuthenticatorSpec) {
	*out = *in
	if in.Interfaces != nil {
		in, out := &in.Interfaces, &out.Interfaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.Authentication.DeepCopyInto(&out.Authentication)
	if in.Configuration != nil {
		in, out := &in.Configuration, &out.Configuration
		*out = new(Config)
		**out = **in
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.TrafficControl != nil {
		in, out := &in.TrafficControl, &out.TrafficControl
		*out = new(TrafficControl)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthenticatorSpec.
func (in *AuthenticatorSpec) DeepCopy() *AuthenticatorSpec {
	if in == nil {
		return nil
	}
	out := new(AuthenticatorSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AuthenticatorStatus) DeepCopyInto(out *AuthenticatorStatus) {
	*out = *in
	if in.Interfaces != nil {
		in, out := &in.Interfaces, &out.Interfaces
		*out = make([]*Interface, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(Interface)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AuthenticatorStatus.
func (in *AuthenticatorStatus) DeepCopy() *AuthenticatorStatus {
	if in == nil {
		return nil
	}
	out := new(AuthenticatorStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Config) DeepCopyInto(out *Config) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Config.
func (in *Config) DeepCopy() *Config {
	if in == nil {
		return nil
	}
	out := new(Config)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Interface) DeepCopyInto(out *Interface) {
	*out = *in
	if in.AuthenticatedClients != nil {
		in, out := &in.AuthenticatedClients, &out.AuthenticatedClients
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Interface.
func (in *Interface) DeepCopy() *Interface {
	if in == nil {
		return nil
	}
	out := new(Interface)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Local) DeepCopyInto(out *Local) {
	*out = *in
	if in.UserFileSecret != nil {
		in, out := &in.UserFileSecret, &out.UserFileSecret
		*out = new(SecretKeyRef)
		**out = **in
	}
	if in.CaCertSecret != nil {
		in, out := &in.CaCertSecret, &out.CaCertSecret
		*out = new(SecretKeyRef)
		**out = **in
	}
	if in.ServerCertSecret != nil {
		in, out := &in.ServerCertSecret, &out.ServerCertSecret
		*out = new(SecretKeyRef)
		**out = **in
	}
	if in.PrivateKeySecret != nil {
		in, out := &in.PrivateKeySecret, &out.PrivateKeySecret
		*out = new(SecretKeyRef)
		**out = **in
	}
	if in.RadiusClientSecret != nil {
		in, out := &in.RadiusClientSecret, &out.RadiusClientSecret
		*out = new(SecretKeyRef)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Local.
func (in *Local) DeepCopy() *Local {
	if in == nil {
		return nil
	}
	out := new(Local)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Ports) DeepCopyInto(out *Ports) {
	*out = *in
	if in.Tcp != nil {
		in, out := &in.Tcp, &out.Tcp
		*out = make([]int, len(*in))
		copy(*out, *in)
	}
	if in.Udp != nil {
		in, out := &in.Udp, &out.Udp
		*out = make([]int, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Ports.
func (in *Ports) DeepCopy() *Ports {
	if in == nil {
		return nil
	}
	out := new(Ports)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Radius) DeepCopyInto(out *Radius) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Radius.
func (in *Radius) DeepCopy() *Radius {
	if in == nil {
		return nil
	}
	out := new(Radius)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretKeyRef) DeepCopyInto(out *SecretKeyRef) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SecretKeyRef.
func (in *SecretKeyRef) DeepCopy() *SecretKeyRef {
	if in == nil {
		return nil
	}
	out := new(SecretKeyRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TrafficControl) DeepCopyInto(out *TrafficControl) {
	*out = *in
	if in.UnprotectedPorts != nil {
		in, out := &in.UnprotectedPorts, &out.UnprotectedPorts
		*out = new(Ports)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TrafficControl.
func (in *TrafficControl) DeepCopy() *TrafficControl {
	if in == nil {
		return nil
	}
	out := new(TrafficControl)
	in.DeepCopyInto(out)
	return out
}
