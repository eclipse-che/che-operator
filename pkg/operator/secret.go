//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
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
			Name:      "self-signed-certificate",
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
