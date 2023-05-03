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

package configgen

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
)

const (
	AppId                   = "authenticator.eapol"
	AuthNamespace           = "authenticator-namespace"
	AuthName                = "authenticator-name"
	AuthenticatorMountPath  = "/config/auth"
	configFile              = "hostapd.conf"
	userFile                = "hostapd.eap_user"
	caFile                  = "1x-ca.pem"
	certFile                = "1x-hostapd.example.com.pem"
	privateKeyFile          = "1x-hostapd.example.com.key"
	radiusClientFile        = "hostapd.radius_clients"
	configMountPath         = "/config"
	configVolumeName        = "config-volume"
	socketsMountPath        = "/var/run/hostapd"
	socketsVolumeName       = "sockets-volume"
	authenticatorVolumeName = "authenticator-volume"
	defaultImage            = "quay.io/openshift-kni/eapol-authenticator:latest"
	disabledSelector        = "no-node"
	disabledReason          = "Disabled_via_config"
	mainCommand             = "/bin/hostapd-start.sh"
	monitorCommand          = "/bin/hostapd-monitor"
)

/* Defaults to avoid excessive reconciliations: */
var defaultFileMode int32 = 420
var terminationGracePeriod int64 = 30
var revisionHistoryLimit int32 = 10
var maxSurge = intstr.FromInt(0)
var maxUnavailable = intstr.FromInt(1)

/* -------------------------------------------- */

type ConfigGenerator struct {
	a11r           *eapolv1.Authenticator
	serviceAccount string
}

func New(a11r *eapolv1.Authenticator, serviceAccount string) *ConfigGenerator {
	return &ConfigGenerator{
		a11r:           a11r,
		serviceAccount: serviceAccount,
	}
}

//go:embed data/hostapd.conf.tmpl
var hostapdConfTemplate string

func (g *ConfigGenerator) ConfigMap() (*corev1.ConfigMap, error) {
	var buffer bytes.Buffer
	tmpl, err := template.New(configFile).Parse(hostapdConfTemplate)
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(&buffer, g.a11r.Spec)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.a11r.Name,
			Namespace: g.a11r.Namespace,
		},
		Data: map[string]string{
			configFile: buffer.String(),
		},
	}
	return cm, nil
}

func (g *ConfigGenerator) Daemonset() *appsv1.DaemonSet {
	nodeSelector := g.a11r.Spec.NodeSelector
	if !g.a11r.Spec.Enabled {
		if nodeSelector == nil {
			nodeSelector = make(map[string]string)
		}
		// Daemonsets do not scale, so use an unsatisfiable node selector
		nodeSelector[disabledSelector] = disabledReason
	}
	ls := map[string]string{"app": AppId, AuthNamespace: g.a11r.Namespace,
		AuthName: g.a11r.Name}
	projectedConfigVolumes := []corev1.VolumeProjection{{
		ConfigMap: &corev1.ConfigMapProjection{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: g.a11r.Name,
			},
			Items: []corev1.KeyToPath{{
				Key:  configFile,
				Path: configFile,
			}},
		},
	}}
	if g.a11r.Spec.Authentication.Local != nil && g.a11r.Spec.Authentication.Local.UserFileSecret != nil {
		secretKey := g.a11r.Spec.Authentication.Local.UserFileSecret.Key
		if secretKey == "" {
			secretKey = userFile
		}
		projectedConfigVolumes = append(projectedConfigVolumes, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: g.a11r.Spec.Authentication.Local.UserFileSecret.Name,
				},
				Items: []corev1.KeyToPath{{
					Key:  secretKey,
					Path: userFile,
				}},
			},
		})
	}
	projectedConfigVolumes = g.appendCertVolume(projectedConfigVolumes)
	projectedConfigVolumes = g.appendRadiusClientVolume(projectedConfigVolumes)
	image := g.a11r.Spec.Image
	if image == "" {
		image = defaultImage
	}

	container := func(name, command string, env []corev1.EnvVar) corev1.Container {
		return corev1.Container{
			Name:  name,
			Image: image,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      configVolumeName,
				MountPath: configMountPath,
			}, {
				Name:      socketsVolumeName,
				MountPath: socketsMountPath,
			}, {
				Name:      authenticatorVolumeName,
				MountPath: AuthenticatorMountPath,
			}},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
				},
			},
			Command: []string{command},
			Env:     env,
			/* Defaults to avoid excessive reconciliations: */
			ImagePullPolicy:          corev1.PullIfNotPresent,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
			/* -------------------------------------------- */
		}
	}

	ifaces := strings.Join(g.a11r.Spec.Interfaces, ",")

	unprotectedTcpList, unprotectedUdpList := g.parsePorts()

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.a11r.Name,
			Namespace: g.a11r.Namespace,
			Labels:    ls,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					NodeSelector:       nodeSelector,
					HostNetwork:        true,
					ServiceAccountName: g.serviceAccount,
					Containers: []corev1.Container{
						container("hostapd", mainCommand,
							[]corev1.EnvVar{{
								Name:  "IFACES",
								Value: ifaces,
							}, {
								Name:  "UNPROTECTED_TCP_PORTS",
								Value: unprotectedTcpList,
							}, {
								Name:  "UNPROTECTED_UDP_PORTS",
								Value: unprotectedUdpList,
							}, {
								Name:  "CONFIG",
								Value: fmt.Sprintf("%s/%s", configMountPath, configFile),
							}}),
						container("hostapd-monitor", monitorCommand,
							[]corev1.EnvVar{{
								Name:  "IFACES",
								Value: ifaces,
							}, {
								Name:  "UNPROTECTED_TCP_PORTS",
								Value: unprotectedTcpList,
							}, {
								Name:  "UNPROTECTED_UDP_PORTS",
								Value: unprotectedUdpList,
							}, {
								Name:      "AUTHENTICATOR_HOST",
								ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.hostIP"}},
							}}),
					},
					Volumes: []corev1.Volume{{
						Name: configVolumeName,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: projectedConfigVolumes,
								/* Defaults to avoid excessive reconciliations: */
								DefaultMode: &defaultFileMode,
								/* -------------------------------------------- */
							},
						},
					}, {
						Name: socketsVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}, {
						Name: authenticatorVolumeName,
						VolumeSource: corev1.VolumeSource{DownwardAPI: &corev1.DownwardAPIVolumeSource{
							Items: []corev1.DownwardAPIVolumeFile{{Path: "labels",
								FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels"}}}}},
					}},
					/* Defaults to avoid excessive reconciliations: */
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriod,
					DNSPolicy:                     "ClusterFirst",
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 "default-scheduler",
					/* -------------------------------------------- */
				},
			},
			/* Defaults to avoid excessive reconciliations: */
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: "RollingUpdate",
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				},
			},
			RevisionHistoryLimit: &revisionHistoryLimit,
			/* -------------------------------------------- */
		},
	}

	return ds
}

func (g *ConfigGenerator) appendCertVolume(volumes []corev1.VolumeProjection) []corev1.VolumeProjection {
	if g.a11r.Spec.Authentication.Local != nil && g.a11r.Spec.Authentication.Local.CaCertSecret != nil {
		caSecretKey := g.a11r.Spec.Authentication.Local.CaCertSecret.Key
		if caSecretKey == "" {
			caSecretKey = caFile
		}
		volumes = append(volumes, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: g.a11r.Spec.Authentication.Local.CaCertSecret.Name,
				},
				Items: []corev1.KeyToPath{{
					Key:  caSecretKey,
					Path: caFile,
				}},
			},
		})
	}
	if g.a11r.Spec.Authentication.Local != nil && g.a11r.Spec.Authentication.Local.ServerCertSecret != nil {
		certSecretKey := g.a11r.Spec.Authentication.Local.ServerCertSecret.Key
		if certSecretKey == "" {
			certSecretKey = certFile
		}
		volumes = append(volumes, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: g.a11r.Spec.Authentication.Local.ServerCertSecret.Name,
				},
				Items: []corev1.KeyToPath{{
					Key:  certSecretKey,
					Path: certFile,
				}},
			},
		})
	}
	if g.a11r.Spec.Authentication.Local != nil && g.a11r.Spec.Authentication.Local.PrivateKeySecret != nil {
		privateKeySecretKey := g.a11r.Spec.Authentication.Local.PrivateKeySecret.Key
		if privateKeySecretKey == "" {
			privateKeySecretKey = privateKeyFile
		}
		volumes = append(volumes, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: g.a11r.Spec.Authentication.Local.PrivateKeySecret.Name,
				},
				Items: []corev1.KeyToPath{{
					Key:  privateKeySecretKey,
					Path: privateKeyFile,
				}},
			},
		})
	}
	return volumes
}

func (g *ConfigGenerator) appendRadiusClientVolume(volumes []corev1.VolumeProjection) []corev1.VolumeProjection {
	if g.a11r.Spec.Authentication.Local != nil && g.a11r.Spec.Authentication.Local.RadiusClientSecret != nil {
		radiusClientSecretKey := g.a11r.Spec.Authentication.Local.RadiusClientSecret.Key
		if radiusClientSecretKey == "" {
			radiusClientSecretKey = radiusClientFile
		}
		volumes = append(volumes, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: g.a11r.Spec.Authentication.Local.RadiusClientSecret.Name,
				},
				Items: []corev1.KeyToPath{{
					Key:  radiusClientSecretKey,
					Path: radiusClientFile,
				}},
			},
		})
	}
	return volumes
}

func (g *ConfigGenerator) parsePorts() (string, string) {
	if g.a11r.Spec.TrafficControl == nil || g.a11r.Spec.TrafficControl.UnprotectedPorts == nil {
		return "", ""
	}
	tcp := strings.Builder{}
	for _, tcpPort := range g.a11r.Spec.TrafficControl.UnprotectedPorts.Tcp {
		if tcp.Len() > 0 {
			tcp.WriteRune(' ')
		}
		tcp.WriteString(strconv.Itoa(tcpPort))
	}
	udp := strings.Builder{}
	for _, udpPort := range g.a11r.Spec.TrafficControl.UnprotectedPorts.Udp {
		if udp.Len() > 0 {
			udp.WriteRune(' ')
		}
		udp.WriteString(strconv.Itoa(udpPort))
	}
	return tcp.String(), udp.String()
}
