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
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newServiceAccount(name string) *corev1.ServiceAccount {
	labels := map[string]string{"app": "che"}
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			Namespace: namespace,
			Labels:    labels,

		},
	}
}

// CreateServiceAccount creates a service account that Che will use to creates workspace objects
// Che service account requires a RoleBinding
func CreateServiceAccount(name string) (*corev1.ServiceAccount) {
	rt := newServiceAccount(name)
	if err := sdk.Create(rt); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create service account : %v", err)
		return nil
	}
	return rt

}