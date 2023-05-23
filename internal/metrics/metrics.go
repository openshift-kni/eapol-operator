/*
Copyright 2023.

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

package metrics

type metric struct {
	Name string
	Help string
}

var (
	Namespace = "authenticator"
	Subsystem = "hostapd"

	AuthSuccess = metric{
		Name: "auth_success_total",
		Help: "total successful authentications for wpa supplicants",
	}

	AuthFailed = metric{
		Name: "auth_failure_total",
		Help: "total failed authentications for wpa supplicants",
	}
)
