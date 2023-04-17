package controllers

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
	. "github.com/openshift-kni/eapol-operator/internal/testutils"
)

func BeOwnedBy(obj metav1.Object) types.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"ObjectMeta": MatchFields(IgnoreExtras, Fields{
			"OwnerReferences": ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal(obj.GetName()),
				"UID":  Equal(obj.GetUID()),
			})),
		}),
	})
}

var _ = Describe("Reconcile", func() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	var a11r *eapolv1.Authenticator
	var cm *corev1.ConfigMap
	var ds *appsv1.DaemonSet
	var key client.ObjectKey

	BeforeEach(func() {
		a11r = NewA11r()
		key = client.ObjectKey{
			Name:      a11r.Name,
			Namespace: a11r.Namespace,
		}
		k8sClient.Delete(ctx, a11r)
		Eventually(func() bool {
			return errors.IsNotFound(k8sClient.Get(ctx, key, a11r))
		}, timeout, interval).Should(BeTrue())
		Expect(k8sClient.Create(ctx, a11r)).To(Succeed())
		a11r = &eapolv1.Authenticator{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, a11r)
		}).Should(BeNil())
	})

	AfterEach(func() {
		if cm != nil {
			k8sClient.Delete(ctx, cm) // Best-effort; ignore any error
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, key, cm))
			}, timeout, interval).Should(BeTrue())
		}
		if ds != nil {
			k8sClient.Delete(ctx, ds)
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, key, ds))
			}, timeout, interval).Should(BeTrue())
		}
		k8sClient.Delete(ctx, a11r)
		Eventually(func() bool {
			return errors.IsNotFound(k8sClient.Get(ctx, key, a11r))
		}, timeout, interval).Should(BeTrue())
	})

	It("should create the daemonset and configmap when Authenticator is first created", func() {
		By("Waiting for ConfigMap creation")
		cm = &corev1.ConfigMap{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, cm)
		}, timeout, interval).Should(Succeed())
		Expect(*cm).To(BeOwnedBy(a11r))

		By("Waiting for DaemonSet creation")
		ds = &appsv1.DaemonSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, ds)
		}, timeout, interval).Should(Succeed())
		Expect(*ds).To(BeOwnedBy(a11r))
	})

	It("should update the daemonset and configmap when Authenticator is updated", func() {
		By("Waiting for object creations")
		cm = &corev1.ConfigMap{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, cm)
		}, timeout, interval).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, key, cm)
		}, timeout, interval).Should(Succeed())
		ds = &appsv1.DaemonSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, ds)
		}, timeout, interval).Should(Succeed())

		Expect(cm.Data["hostapd.conf"]).NotTo(ContainSubstring("\neap_server=1\n"))
		Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainLocaluserVolumeMount())

		By("Updating the Authenticator's authentication type")
		SetupUserFileAuth(a11r, "local-secret", "")
		Expect(k8sClient.Update(ctx, a11r)).To(Succeed())

		Eventually(func() string {
			Expect(k8sClient.Get(ctx, key, cm)).To(Succeed())
			return cm.Data["hostapd.conf"]
		}).Should(ContainSubstring("\neap_server=1\n"))

		Eventually(func() []corev1.VolumeProjection {
			Expect(k8sClient.Get(ctx, key, ds)).To(Succeed())
			return ds.Spec.Template.Spec.Volumes[0].Projected.Sources
		}).Should(
			ContainLocaluserProjection("local-secret"))
	})
	It("should disable the daemonset when configured to so so", func() {
		By("Waiting for object creations")
		cm = &corev1.ConfigMap{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, cm)
		}, timeout, interval).Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, key, cm)
		}, timeout, interval).Should(Succeed())
		ds = &appsv1.DaemonSet{}
		Eventually(func() error {
			return k8sClient.Get(ctx, key, ds)
		}, timeout, interval).Should(Succeed())

		Expect(ds.Spec.Template.Spec.NodeSelector).NotTo(HaveKey("no-node"))

		Expect(ds.Spec.Template.Spec.Containers[0].VolumeMounts).NotTo(ContainLocaluserVolumeMount())

		By("disabling the Authenticator")
		a11r.Spec.Enabled = false
		Expect(k8sClient.Update(ctx, a11r)).To(Succeed())

		Eventually(func() map[string]string {
			Expect(k8sClient.Get(ctx, key, ds)).To(Succeed())
			return ds.Spec.Template.Spec.NodeSelector
		}).Should(HaveKey("no-node"))
	})
})
