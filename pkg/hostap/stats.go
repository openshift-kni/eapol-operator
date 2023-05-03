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

package hostap

import (
	authmetrics "github.com/openshift-kni/eapol-operator/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var labels = []string{"interface"}

var stats = metrics{
	authSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: authmetrics.Namespace,
		Subsystem: authmetrics.Subsystem,
		Name:      authmetrics.AuthSuccess.Name,
		Help:      authmetrics.AuthSuccess.Help,
	}, labels),

	authFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: authmetrics.Namespace,
		Subsystem: authmetrics.Subsystem,
		Name:      authmetrics.AuthFailed.Name,
		Help:      authmetrics.AuthFailed.Help,
	}, labels),
}

type metrics struct {
	authSuccess *prometheus.GaugeVec
	authFailed  *prometheus.CounterVec
}

func init() {
	prometheus.MustRegister(stats.authSuccess)
	prometheus.MustRegister(stats.authFailed)
}

func (m *metrics) Authenticated(iface string) {
	m.authSuccess.WithLabelValues(iface).Inc()
}

func (m *metrics) DeAuthenticated(iface string) {
	m.authSuccess.WithLabelValues(iface).Dec()
}

func (m *metrics) AuthFailed(iface string) {
	m.authFailed.WithLabelValues(iface).Inc()
}
