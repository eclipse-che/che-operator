//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

var ClusterRoleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRole{}, "TypeMeta", "ObjectMeta"),
}

func SyncClusterRoleToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	policyRule []rbac.PolicyRule) (bool, error) {

	crSpec := getClusterRoleSpec(deployContext, name, policyRule)
	return Sync(deployContext, crSpec, ClusterRoleDiffOpts)
}

func getClusterRoleSpec(deployContext *chetypes.DeployContext, name string, policyRule []rbac.PolicyRule) *rbac.ClusterRole {
	labels := GetLabels(defaults.GetCheFlavor())
	clusterRole := &rbac.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Rules: policyRule,
	}

	return clusterRole
}
