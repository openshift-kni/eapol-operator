package testutils

import (
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	eapolv1 "github.com/openshift-kni/eapol-operator/api/v1"
)

func NewA11r() *eapolv1.Authenticator {
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
			Enabled:    true,
			Interfaces: []string{"eth0"},
			Authentication: eapolv1.Auth{
				Radius: &eapolv1.Radius{},
			},
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

func SetupUserFileAuth(a11r *eapolv1.Authenticator, secretName, key string) {
	a11r.Spec.Authentication.Radius = nil
	a11r.Spec.Authentication.Local = &eapolv1.Local{
		UserFileSecret: &eapolv1.SecretKeyRef{
			Name: secretName,
			Key:  key,
		},
	}
}

func SetupRadiusAuth(a11r *eapolv1.Authenticator) {
	a11r.Spec.Authentication.Local = nil
	a11r.Spec.Authentication.Radius = &eapolv1.Radius{}
}
