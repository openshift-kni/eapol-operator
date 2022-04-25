package controllers

import (
	"strings"
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
)

func makeReconciler() *AuthenticatorReconciler {
	return &AuthenticatorReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}
}

func makeA11r() *eapolv1.Authenticator {
	a11r := &eapolv1.Authenticator{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eapol.eapol.openshift.io/v1",
			Kind:       "Authenticator",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "authenticator",
			Namespace: "default",
		},
		Spec: eapolv1.AuthenticatorSpec{
			Enabled:        true,
			Interfaces:     []string{"eth0"},
			Authentication: eapolv1.Auth{},
		},
	}
	return a11r
}

func ContainLocaluserVolumeMount() types.GomegaMatcher {
	return ContainElement(MatchFields(IgnoreExtras, Fields{
		"Name":      Equal("local-users"),
		"MountPath": Equal("/etc/local-users"),
	}))
}

func ContainLocaluserProjection(secretName string) types.GomegaMatcher {
	return ContainElement(MatchFields(IgnoreExtras, Fields{
		"Secret": PointTo(
			MatchFields(IgnoreExtras, Fields{
				"LocalObjectReference": MatchFields(IgnoreExtras, Fields{
					"Name": Equal(secretName),
				}),
			})),
	}))
}

var _ = Describe("daemonsetForAuthenticator", func() {
	var a11r *eapolv1.Authenticator
	r := AuthenticatorReconciler{}
	BeforeEach(func() {
		a11r = makeA11r()
	})
	It("should generate a DaemonSet with the right name", func() {
		ds := r.daemonsetForAuthenticator(a11r)
		Expect(ds.ObjectMeta.Name).To(Equal(a11r.Name))
		Expect(ds.ObjectMeta.Namespace).To(Equal(a11r.Namespace))
	})
	It("should configure local-user secret projection when local-auth is configured", func() {
		a11r.Spec.Authentication.LocalSecret = "localsecretname"
		ds := r.daemonsetForAuthenticator(a11r)
		Expect(ds.Spec.Template.Spec.Volumes[0].Projected.Sources).To(ContainLocaluserProjection("localsecretname"))
	})
	It("should not configure local-user secret projection when local-auth is not configured", func() {
		ds := r.daemonsetForAuthenticator(a11r)
		Expect(ds.Spec.Template.Spec.Volumes[0].Projected.Sources).NotTo(ContainLocaluserProjection("localsecretname"))
	})
})

var _ = Describe("configmapForAuthenticator", func() {
	var a11r *eapolv1.Authenticator
	r := AuthenticatorReconciler{}
	BeforeEach(func() {
		a11r = makeA11r()
	})
	It("should create a ConfigMap with the right name", func() {
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Name).To(Equal(a11r.Name))
		Expect(cm.Namespace).To(Equal(a11r.Namespace))
	})
	It("should create the right Interface entry with a single interface input", func() {
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\ninterface=eth0\n"))
	})
	It("should create the right Interface entry with multiple interface input", func() {
		a11r.Spec.Interfaces = strings.Split("nic1,nic2,nic3", ",")
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\ninterface=nic1,nic2,nic3\n"))
	})
	It("should configure the eap_reauth_period correctly when no configuration is provided", func() {
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_reauth_period=3600\n"))
	})
	It("should configure the eap_reauth_period correctly when a specific configuration is provided", func() {
		a11r.Spec.Configuration = &eapolv1.Config{
			EapReauthPeriod: 42,
		}
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_reauth_period=42\n"))
	})
	It("should configure the eap_reauth_period correctly when a zeroed configuration is provided", func() {
		a11r.Spec.Configuration = &eapolv1.Config{
			EapReauthPeriod: 0,
		}
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_reauth_period=0\n"))
	})
	It("should configure the internal EAP server when local-auth is configured", func() {
		a11r.Spec.Authentication.LocalSecret = "localsecret"
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_server=1\n"))
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_user_file=/config/hostapd.eap_user\n"))
	})
	It("should configure the RADIUS server when configured", func() {
		a11r.Spec.Authentication.Radius = &eapolv1.Radius{
			AuthServer: "1.1.1.1",
			AuthPort:   8080,
		}
		cm, err := r.configmapForAuthenticator(a11r)
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\nauth_server_addr=1.1.1.1\n"))
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\nauth_server_port=8080\n"))
	})
})

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
		a11r = makeA11r()
		key = client.ObjectKey{
			Name:      a11r.Name,
			Namespace: a11r.Namespace,
		}
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
		a11r.Spec.Authentication.LocalSecret = "local-secret"
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
})
