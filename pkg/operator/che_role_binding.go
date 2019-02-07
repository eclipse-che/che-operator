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
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newRoleBinding(kind string,name string, serviceAccountName string, roleName string, roleKind string) *rbac.RoleBinding {
	return &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			Namespace: namespace,
			Labels:    cheLabels,

		},
		Subjects: []rbac.Subject{
			{
				Kind: "ServiceAccount",
				Name: serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

// CreateRoleBinding creates a role binding for che related service accounts that enables it to access/edit resources in a target (current) namespace
func CreateRoleBinding(kind string, name string, serviceAccountName string, roleName string, roleKind string) *rbac.RoleBinding {
	rb := newRoleBinding(kind, name, serviceAccountName, roleName, roleKind)
	err := sdk.Create(rb)
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create %s RoleBinding: %v", name, err)
		return nil
	}
	return rb
}
