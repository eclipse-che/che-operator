//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var RollBindingDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.RoleBinding{}, "TypeMeta", "ObjectMeta"),
}

func SyncRoleBindingToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string) (bool, error) {

	rbSpec := getRoleBindingSpec(deployContext, name, serviceAccountName, roleName, roleKind)
	return Sync(deployContext, rbSpec, RollBindingDiffOpts)
}

func getRoleBindingSpec(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string) *rbac.RoleBinding {

	labels := GetLabels(defaults.GetCheFlavor())
	roleBinding := &rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: deployContext.CheCluster.Namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     roleName,
			Kind:     roleKind,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return roleBinding
}
