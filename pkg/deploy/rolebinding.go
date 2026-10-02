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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncRoleBindingToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	roleName string,
	roleKind string) error {

	rbSpec := getRoleBindingSpec(deployContext, name, serviceAccountName, roleName, roleKind)

	if err := controllerutil.SetControllerReference(deployContext.CheCluster, rbSpec, deployContext.ClusterAPI.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for RoleBinding %s/%s: %w", rbSpec.Namespace, rbSpec.Name, err)
	}

	if err := deployContext.ClusterAPI.ClientWrapper.Sync(
		deployContext.Context,
		rbSpec,
		&k8sclient.SyncOptions{DiffOpts: diffs.RoleBinding},
	); err != nil {
		return fmt.Errorf("failed to sync RoleBinding %s/%s: %w", rbSpec.Namespace, rbSpec.Name, err)
	}

	return nil
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
