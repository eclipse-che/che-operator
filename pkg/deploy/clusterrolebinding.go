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
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ClusterRoleBindingDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRoleBinding{}, "TypeMeta", "ObjectMeta"),
}

func SyncClusterRoleBindingToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	clusterRoleName string) (bool, error) {

	crbSpec := getClusterRoleBindingSpec(deployContext, name, serviceAccountName, deployContext.CheCluster.Namespace, clusterRoleName)
	return Sync(deployContext, crbSpec, ClusterRoleBindingDiffOpts)
}

func getClusterRoleBindingSpec(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	serviceAccountNamespace string,
	clusterRoleName string) *rbac.ClusterRoleBinding {

	labels := GetLabels(defaults.GetCheFlavor())
	clusterRoleBinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: serviceAccountNamespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
		},
	}

	return clusterRoleBinding
}
