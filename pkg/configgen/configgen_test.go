package configgen

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
	. "github.com/openshift-kni/eapol-operator/internal/testutils"
)

var _ = Describe("Daemonset", func() {
	var cfggen *ConfigGenerator
	BeforeEach(func() {
		cfggen = New(NewA11r())
	})
	It("should generate a DaemonSet with the right name", func() {
		ds := cfggen.Daemonset()
		Expect(ds.ObjectMeta.Name).To(Equal(cfggen.a11r.Name))
		Expect(ds.ObjectMeta.Namespace).To(Equal(cfggen.a11r.Namespace))
	})
	It("should configure local-user secret projection when local-auth is configured", func() {
		SetupUserFileAuth(cfggen.a11r, "localsecretname", "hostapd.eap_user")
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Volumes[0].Projected.Sources).To(ContainLocaluserProjection("localsecretname"))
	})
	It("should not configure local-user secret projection when local-auth is not configured", func() {
		cfggen.a11r.Spec.Authentication.Local = nil
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Volumes[0].Projected.Sources).NotTo(ContainLocaluserProjection("localsecretname"))
	})
	It("should not configure local-user secret projection when user-secret is not configured", func() {
		cfggen.a11r.Spec.Authentication.Local = &eapolv1.Local{}
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Volumes[0].Projected.Sources).NotTo(ContainLocaluserProjection("localsecretname"))
	})
	It("should set a reasonable image name default", func() {
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Containers[0].Image).NotTo(BeEmpty())
	})
	It("should override the image name when specified", func() {
		cfggen.a11r.Spec.Image = "image:override"
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal("image:override"))
	})
	It("should set a restrictive node selector when disabled", func() {
		cfggen.a11r.Spec.Enabled = true
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.NodeSelector).NotTo(HaveKey("no-node"))
		cfggen.a11r.Spec.Enabled = false
		ds = cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.NodeSelector).To(HaveKey("no-node"))
	})
	It("should maintain a user-supplied node selector", func() {
		cfggen.a11r.Spec.Enabled = true
		cfggen.a11r.Spec.NodeSelector = map[string]string{"nodeType": "auth"}
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.NodeSelector).To(HaveKey("nodeType"))
		Expect(ds.Spec.Template.Spec.NodeSelector).NotTo(HaveKey("no-node"))
		cfggen.a11r.Spec.Enabled = false
		ds = cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.NodeSelector).To(HaveKey("nodeType"))
		Expect(ds.Spec.Template.Spec.NodeSelector).To(HaveKey("no-node"))
	})
	It("should contain one monitoring container per defined interface", func() {
		cfggen.a11r.Spec.Interfaces = []string{"iface1"}
		ds := cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Containers).To(ContainElements(
			MatchFields(IgnoreExtras, Fields{
				"Name": Equal("monitor-iface1"),
			}),
		))
		cfggen.a11r.Spec.Interfaces = []string{"iface2", "iface3"}
		ds = cfggen.Daemonset()
		Expect(ds.Spec.Template.Spec.Containers).To(ContainElements(
			MatchFields(IgnoreExtras, Fields{
				"Name": Equal("monitor-iface2"),
			}),
			MatchFields(IgnoreExtras, Fields{
				"Name": Equal("monitor-iface3"),
			}),
		))
	})
})

var _ = Describe("ConfigMap", func() {
	var cfggen *ConfigGenerator
	BeforeEach(func() {
		cfggen = New(NewA11r())
	})
	It("should create a ConfigMap with the right name", func() {
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Name).To(Equal(cfggen.a11r.Name))
		Expect(cm.Namespace).To(Equal(cfggen.a11r.Namespace))
	})
	It("should create the right Interface entry with a single interface input", func() {
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\ninterface=eth0\n"))
	})
	It("should create the right Interface entry with multiple interface input", func() {
		cfggen.a11r.Spec.Interfaces = strings.Split("nic1,nic2,nic3", ",")
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\ninterface=nic1,nic2,nic3\n"))
	})
	It("should configure the eap_reauth_period correctly when no configuration is provided", func() {
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_reauth_period=3600\n"))
	})
	It("should configure the eap_reauth_period correctly when a specific configuration is provided", func() {
		cfggen.a11r.Spec.Configuration = &eapolv1.Config{
			EapReauthPeriod: 42,
		}
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_reauth_period=42\n"))
	})
	It("should configure the eap_reauth_period correctly when a zeroed configuration is provided", func() {
		cfggen.a11r.Spec.Configuration = &eapolv1.Config{
			EapReauthPeriod: 0,
		}
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_reauth_period=0\n"))
	})
	It("should configure the internal EAP server when local-auth is configured", func() {
		SetupUserFileAuth(cfggen.a11r, "localsecret", "")
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_server=1\n"))
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\neap_user_file=/config/hostapd.eap_user\n"))
	})
	It("should configure the RADIUS server when configured", func() {
		cfggen.a11r.Spec.Authentication.Radius = &eapolv1.Radius{
			AuthServer: "1.1.1.1",
			AuthPort:   8080,
		}
		cm, err := cfggen.ConfigMap()
		Expect(err).NotTo(HaveOccurred())
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\nauth_server_addr=1.1.1.1\n"))
		Expect(cm.Data["hostapd.conf"]).To(ContainSubstring("\nauth_server_port=8080\n"))
	})
})
