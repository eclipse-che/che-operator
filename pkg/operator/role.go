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

func newRole(name string, resources []string, verbs []string) *rbac.Role {
	labels := map[string]string{"app": "che"}
	return &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Rules: []rbac.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: resources,
				Verbs:     verbs,
			},
		},
	}
}

func CreateNewRole(name string, resources []string, verbs []string) *rbac.Role {
	role := newRole(name, resources, verbs)
	if err := sdk.Create(role); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+name+" role : %v", err)
		return nil
	}
	return role
}
