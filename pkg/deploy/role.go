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
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var roleDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta"),
	cmpopts.IgnoreFields(rbac.PolicyRule{}, "ResourceNames", "NonResourceURLs"),
}

func SyncRoleToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	policyRule []rbac.PolicyRule) error {

	roleSpec := getRoleSpec(deployContext, name, policyRule)

	if err := controllerutil.SetControllerReference(deployContext.CheCluster, roleSpec, deployContext.ClusterAPI.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for Role %s/%s: %w", roleSpec.Namespace, roleSpec.Name, err)
	}

	if err := deployContext.ClusterAPI.ClientWrapper.Sync(
		deployContext.Context,
		roleSpec,
		&k8sclient.SyncOptions{DiffOpts: roleDiffOpts},
	); err != nil {
		return fmt.Errorf("failed to sync Role %s/%s: %w", roleSpec.Namespace, roleSpec.Name, err)
	}

	return nil
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
