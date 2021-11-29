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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ViewRoleName role to get k8s object needed for Workspace components(metrics plugin, Che terminals, tasks etc.)
	ViewRoleName = "view"
	// ExecRoleName - role name to create Che terminals and tasks in the workspace.
	ExecRoleName = "exec"
)

var roleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "ResourceNames", "NonResourceURLs"),
}

func SyncExecRoleToCluster(deployContext *DeployContext) (bool, error) {
	execPolicyRule := []rbac.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods/exec",
			},
			Verbs: []string{
				"*",
			},
		},
	}
	return SyncRoleToCluster(deployContext, ExecRoleName, execPolicyRule)
}

func SyncViewRoleToCluster(deployContext *DeployContext) (bool, error) {
	viewPolicyRule := []rbac.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"list", "get",
			},
		},
		{
			APIGroups: []string{
				"metrics.k8s.io",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"list", "get", "watch",
			},
		},
	}
	return SyncRoleToCluster(deployContext, ViewRoleName, viewPolicyRule)
}

func SyncRoleToCluster(
	deployContext *DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (bool, error) {

	roleSpec := getRoleSpec(deployContext, name, policyRule)
	return Sync(deployContext, roleSpec, roleDiffOpts)
}

func getRoleSpec(deployContext *DeployContext, name string, policyRule []rbac.PolicyRule) *rbac.Role {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	role := &rbac.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Rules: policyRule,
	}

	return role
}
