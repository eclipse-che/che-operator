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

var crDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRole{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "ResourceNames", "NonResourceURLs"),
}

func SyncClusterRoleToCluster(
	deployContext *DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (bool, error) {

	crSpec := getClusterRoleSpec(deployContext, name, policyRule)
	return Sync(deployContext, crSpec, crDiffOpts)
}

func SyncClusterRoleAndFinalizerToCluster(
	deployContext *DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (bool, error) {

	finalizer := GetFinalizerName(strings.ToLower(name) + ".clusterrole")
	crSpec := getClusterRoleSpec(deployContext, name, policyRule)
	return SyncWithFinalizer(deployContext, crSpec, crbDiffOpts, finalizer)
}

func getClusterRoleSpec(deployContext *DeployContext, name string, policyRule []rbac.PolicyRule) *rbac.ClusterRole {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	clusterRole := &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Rules: policyRule,
	}

	return clusterRole
}
