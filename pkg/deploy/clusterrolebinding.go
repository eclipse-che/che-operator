//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var crbDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRoleBinding{}, "TypeMeta", "ObjectMeta"),
}

func SyncClusterRoleBindingToCluster(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	clusterRoleName string) (bool, error) {

	crbSpec := getClusterRoleBindingSpec(deployContext, name, serviceAccountName, clusterRoleName)
	return Sync(deployContext, crbSpec, crbDiffOpts)
}

func SyncClusterRoleBindingAndFinalizerToCluster(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	clusterRoleName string) (bool, error) {

	finalizer := GetFinalizerName(strings.ToLower(name) + ".clusterrolebinding")
	crbSpec := getClusterRoleBindingSpec(deployContext, name, serviceAccountName, clusterRoleName)
	return SyncWithFinalizer(deployContext, crbSpec, crbDiffOpts, finalizer)
}

func getClusterRoleBindingSpec(
	deployContext *DeployContext,
	name string,
	serviceAccountName string,
	roleName string) *rbac.ClusterRoleBinding {

	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	clusterRoleBinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
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
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
		},
	}

	return clusterRoleBinding
}
