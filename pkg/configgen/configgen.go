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
	"html/template"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
)

const (
	appId             = "authenticator.eapol"
	configFile        = "hostapd.conf"
	userFile          = "hostapd.eap_user"
	configMountPath   = "/config"
	configVolumeName  = "config-volume"
	socketsMountPath  = "/var/run/hostapd"
	socketsVolumeName = "sockets-volume"
	defaultImage      = "quay.io/openshift-kni/eapol-authenticator:latest"
	disabledSelector  = "no-node"
	disabledReason    = "Disabled_via_config"
	initCommand       = "/bin/hostapd-init.sh"
	cliCommand        = "/bin/hostapd-cli.sh"
)

type ConfigGenerator struct {
	a11r *eapolv1.Authenticator
}

func New(a11r *eapolv1.Authenticator) *ConfigGenerator {
	return &ConfigGenerator{
		a11r: a11r,
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
	ls := map[string]string{"app": appId, appId: g.a11r.Name}
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
	image := g.a11r.Spec.Image
	if image == "" {
		image = defaultImage
	}

	container := func(name, command string) corev1.Container {
		c := corev1.Container{
			Name:  name,
			Image: image,
			VolumeMounts: []corev1.VolumeMount{{
				Name:      configVolumeName,
				MountPath: configMountPath,
			}, {
				Name:      socketsVolumeName,
				MountPath: socketsMountPath,
			}},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
				},
			},
		}
		if command != "" {
			c.Command = []string{command}
		}
		return c
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      g.a11r.Name,
			Namespace: g.a11r.Namespace,
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
					NodeSelector: nodeSelector,
					HostNetwork:  true,
					InitContainers: []corev1.Container{
						container("iface-init", initCommand),
					},
					Containers: []corev1.Container{
						container("hostapd", ""),
						container("hostapd-cli", cliCommand),
					},
					Volumes: []corev1.Volume{{
						Name: configVolumeName,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: projectedConfigVolumes,
							},
						},
					}, {
						Name: socketsVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					}},
				},
			},
		},
	}
	return ds
}
