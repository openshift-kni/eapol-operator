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
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-test/deep"
	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
	"github.com/openshift-kni/eapol-operator/pkg/configgen"
)

const authenticatorRbacPathController = "./bindata/deployment/authenticator-rbac"

var AuthenticatorRbacPath = authenticatorRbacPathController

// AuthenticatorReconciler reconciles a Authenticator object
type AuthenticatorReconciler struct {
	client.Client
	rbacResources *resources
	Scheme        *runtime.Scheme
}

//+kubebuilder:rbac:groups=eapol.eapol.openshift.io,resources=authenticators,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=eapol.eapol.openshift.io,resources=authenticators/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=eapol.eapol.openshift.io,resources=authenticators/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=create;delete;get;update;patch;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

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

	cfggen := configgen.New(a11r, r.rbacResources.serviceAccount.Name)

	// Check if the configmap already exists
	newCm, err := cfggen.ConfigMap()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to generate ConfigMap content: %e", err)
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

	err = r.syncRbacResources(ctx, a11r, req.Namespace)
	if err != nil {
		log.Error(err, "Failed to sync authenticator rbac resources")
		return ctrl.Result{}, err
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
		// Daemonset was found; Update contents if changed
		if diff := deep.Equal(ds.Spec, newDs.Spec); diff != nil {
			log.Info(fmt.Sprintf("Current DaemonSet differs from expected: %v", diff))
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

func (r *AuthenticatorReconciler) syncRbacResources(ctx context.Context, owner *eapolv1.Authenticator, namespace string) error {
	_, err := r.getServiceAccount(ctx, namespace)
	if errors.IsNotFound(err) {
		r.rbacResources.serviceAccount.Namespace = namespace
		r.rbacResources.serviceAccount.ResourceVersion = ""
		err = r.createOwned(ctx, owner, r.rbacResources.serviceAccount)
		if err != nil {
			return fmt.Errorf("error creating authenticator service account: %v, err: %v",
				r.rbacResources.serviceAccount, err)
		}
	} else if err != nil {
		return err
	}
	_, err = r.getRole(ctx, namespace)
	if errors.IsNotFound(err) {
		r.rbacResources.role.Namespace = namespace
		r.rbacResources.role.ResourceVersion = ""
		err = r.createOwned(ctx, owner, r.rbacResources.role)
		if err != nil {
			return fmt.Errorf("error creating authenticator role: %v, err: %v",
				r.rbacResources.role, err)
		}
	} else if err != nil {
		return err
	}
	_, err = r.getRoleBinding(ctx, namespace)
	if errors.IsNotFound(err) {
		r.rbacResources.roleBinding.Namespace = namespace
		r.rbacResources.roleBinding.ResourceVersion = ""
		r.rbacResources.roleBinding.Subjects[0].Namespace = namespace
		err = r.createOwned(ctx, owner, r.rbacResources.roleBinding)
		if err != nil {
			return fmt.Errorf("error creating authenticator role binding: %v, err: %v",
				r.rbacResources.roleBinding, err)
		}
	} else if err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AuthenticatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	rbacResources, err := retrieveResources(AuthenticatorRbacPath)
	if err != nil {
		return err
	}
	r.rbacResources = rbacResources
	return ctrl.NewControllerManagedBy(mgr).
		For(&eapolv1.Authenticator{}).
		Owns(&appsv1.DaemonSet{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.RoleBinding{}).
		Owns(&rbacv1.Role{}).
		Complete(r)
}

func (r *AuthenticatorReconciler) createOwned(ctx context.Context, owner, obj client.Object, opts ...client.CreateOption) error {
	ctrl.SetControllerReference(owner, obj, r.Scheme)
	return r.Create(ctx, obj, opts...)
}

func (r *AuthenticatorReconciler) getServiceAccount(ctx context.Context,
	namespace string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace,
		Name: r.rbacResources.serviceAccount.Name}, sa)
	return sa, err
}

func (r *AuthenticatorReconciler) getRole(ctx context.Context,
	namespace string) (*rbacv1.Role, error) {
	role := &rbacv1.Role{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace,
		Name: r.rbacResources.role.Name}, role)
	return role, err
}

func (r *AuthenticatorReconciler) getRoleBinding(ctx context.Context,
	namespace string) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Namespace: namespace,
		Name: r.rbacResources.roleBinding.Name}, rb)
	return rb, err
}
