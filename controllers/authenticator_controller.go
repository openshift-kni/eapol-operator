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
	"context"
	_ "embed"
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
	"github.com/openshift-kni/eapol-operator/pkg/configgen"
)

// AuthenticatorReconciler reconciles a Authenticator object
type AuthenticatorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

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

	cfggen := configgen.New(a11r)

	// Check if the configmap already exists
	newCm, err := cfggen.ConfigMap()
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
	newDs := cfggen.Daemonset()
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
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

func (r *AuthenticatorReconciler) createOwned(ctx context.Context, owner, obj client.Object, opts ...client.CreateOption) error {
	ctrl.SetControllerReference(owner, obj, r.Scheme)
	return r.Create(ctx, obj, opts...)
}
