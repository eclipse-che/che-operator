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

var roleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "ResourceNames", "NonResourceURLs"),
}

func SyncRoleToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (bool, error) {

	roleSpec := getRoleSpec(deployContext, name, policyRule)
	return Sync(deployContext, roleSpec, roleDiffOpts)
}

func getRoleSpec(deployContext *chetypes.DeployContext, name string, policyRule []rbac.PolicyRule) *rbac.Role {
	labels := GetLabels(defaults.GetCheFlavor())
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
