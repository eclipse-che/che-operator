package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newSecret() *corev1.Secret {
	cert := util.GetSelfSignedCert()
	labels := map[string]string{"app": "che"}
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "self-signed-cert",
			Namespace: namespace,
			Labels:    labels,

		},
		Data: map[string][]byte{
			"ca.crt": cert,
		},
	}

}

func CreateCertSecret() (*corev1.Secret) {
	secret := newSecret()
	if err := sdk.Create(secret); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create Che secret : %v", err)
		return nil
	}
	return secret
}
