//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func SyncClusterRoleToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	policyRule []rbac.PolicyRule) error {

	crSpec := getClusterRoleSpec(deployContext, name, policyRule)

	if err := deployContext.ClusterAPI.ClientWrapper.Sync(
		deployContext.Context,
		crSpec,
		&k8sclient.SyncOptions{DiffOpts: diffs.ClusterRole},
	); err != nil {
		return fmt.Errorf("failed to sync ClusterRole %s: %w", crSpec.Name, err)
	}

	return nil
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
