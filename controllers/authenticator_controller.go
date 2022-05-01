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

package controllers

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
)

// AuthenticatorReconciler reconciles a Authenticator object
type AuthenticatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	appId            = "authenticator.eapol"
	configFile       = "hostapd.conf"
	userFile         = "hostapd.eap_user"
	configMountPath  = "/config"
	configVolumeName = "config-volume"
	defaultImage     = "quay.io/openshift-kni/eapol-authenticator:latest"
)

//+kubebuilder:rbac:groups=eapol.eapol.openshift.io,resources=authenticators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=eapol.eapol.openshift.io,resources=authenticators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=eapol.eapol.openshift.io,resources=authenticators/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Authenticator object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *AuthenticatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	a11r := &eapolv1.Authenticator{}
	err := r.Get(ctx, req.NamespacedName, a11r)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Authenticator was deleted")
			// No other action needed: k8s GC will clean up all owned objects
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if the configmap already exists
	newCm, err := r.configmapForAuthenticator(a11r)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("Failed to generate ConfigMap content: %e", err)
	}

	cm := &corev1.ConfigMap{}
	err = r.Get(ctx, req.NamespacedName, cm)
	if err != nil && errors.IsNotFound(err) {
		// Configmap not found; create it
		log.Info("Creating a new ConfigMap")
		err = r.createOwned(ctx, a11r, newCm)
		if err != nil {
			log.Error(err, "Failed to create new ConfigMap")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get ConfigMap")
		return ctrl.Result{}, err
	} else {
		// ConfigMap was found; Update contents if changed
		if !reflect.DeepEqual(cm.Data, newCm.Data) {
			cm.Data = newCm.Data
			log.Info("Updating ConfigMap")
			err = r.Update(ctx, newCm)
			if err != nil {
				log.Error(err, "Failed to update ConfigMap")
				return ctrl.Result{}, err
			}
			// TODO: Signal the DS to restart and/or reload its config?
			// or does the pod watch for changes and just do it?
		}
	}

	// Check if the daemonset already exists
	newDs := r.daemonsetForAuthenticator(a11r)
	ds := &appsv1.DaemonSet{}
	err = r.Get(ctx, req.NamespacedName, ds)
	if err != nil && errors.IsNotFound(err) {
		// Daemonset not found; create it
		log.Info("Creating a new DaemonSet")
		err = r.createOwned(ctx, a11r, newDs)
		if err != nil {
			log.Error(err, "Failed to create new Daemonset")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Daemonset")
		return ctrl.Result{}, err
	} else {
		// Daemonset was found; Take action if it changed?
		if !reflect.DeepEqual(ds.Spec, newDs.Spec) {
			ds.Spec = newDs.Spec
			log.Info("Updating DaemonSet")
			err = r.Update(ctx, ds)
			if err != nil {
				log.Error(err, "Failed to update Daemonset")
				return ctrl.Result{}, err
			}
			// TODO: Restart or signal the DS pod?  Volumes may have changed
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthenticatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&eapolv1.Authenticator{}).
		Complete(r)
}

func (r *AuthenticatorReconciler) createOwned(ctx context.Context, owner, obj client.Object, opts ...client.CreateOption) error {
	ctrl.SetControllerReference(owner, obj, r.Scheme)
	return r.Create(ctx, obj, opts...)
}

func logObjKeys(obj metav1.Object) []interface{} {
	return []interface{}{"Namespace", obj.GetNamespace(), "Name", obj.GetName()}
}

//go:embed data/hostapd.conf.tmpl
var hostapdConfTemplate string

func (r *AuthenticatorReconciler) configmapForAuthenticator(a11r *eapolv1.Authenticator) (*corev1.ConfigMap, error) {
	var buffer bytes.Buffer
	tmpl, err := template.New(configFile).Parse(hostapdConfTemplate)
	if err != nil {
		return nil, err
	}
	err = tmpl.Execute(&buffer, a11r.Spec)
	if err != nil {
		return nil, err
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a11r.Name,
			Namespace: a11r.Namespace,
		},
		Data: map[string]string{
			configFile: buffer.String(),
		},
	}
	return cm, nil
}

func (r *AuthenticatorReconciler) daemonsetForAuthenticator(a11r *eapolv1.Authenticator) *appsv1.DaemonSet {
	ls := map[string]string{"app": appId, appId: a11r.Name}
	projectedConfigVolumes := []corev1.VolumeProjection{{
		ConfigMap: &corev1.ConfigMapProjection{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: a11r.Name,
			},
			Items: []corev1.KeyToPath{{
				Key:  configFile,
				Path: configFile,
			}},
		},
	}}
	if a11r.Spec.Authentication.Local != nil && a11r.Spec.Authentication.Local.UserFileSecret != nil {
		secretKey := a11r.Spec.Authentication.Local.UserFileSecret.Key
		if secretKey == "" {
			secretKey = userFile
		}
		projectedConfigVolumes = append(projectedConfigVolumes, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: a11r.Spec.Authentication.Local.UserFileSecret.Name,
				},
				Items: []corev1.KeyToPath{{
					Key:  secretKey,
					Path: userFile,
				}},
			},
		})
	}
	image := a11r.Spec.Image
	if image == "" {
		image = defaultImage
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a11r.Name,
			Namespace: a11r.Namespace,
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
					HostNetwork: true,
					Containers: []corev1.Container{{
						Name:  "hostapd",
						Image: image,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      configVolumeName,
							MountPath: configMountPath,
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: configVolumeName,
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								Sources: projectedConfigVolumes,
							},
						},
					}},
				},
			},
		},
	}
	return ds
}
