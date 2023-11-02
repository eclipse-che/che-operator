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

package containerbuild

import (
	"context"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"testing"
)

func TestContainerBuildReconciler(t *testing.T) {
	dwSA := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DevWorkspaceServiceAccountName,
			Namespace: "eclipse-che",
		},
	}
	ctx := test.GetDeployContext(nil, []runtime.Object{dwSA})
	containerBuildReconciler := NewContainerBuildReconciler()

	_, done, err := containerBuildReconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	// Enable Container build capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc"}
	err = ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	_, done, err = containerBuildReconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetUserSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.getFinalizerName()))

	// Disable Container build capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(true)
	err = ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	_, done, err = containerBuildReconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetUserSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.getFinalizerName()))
}

func TestSyncAndRemoveRBAC(t *testing.T) {
	dwSA := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DevWorkspaceServiceAccountName,
			Namespace: "eclipse-che",
		},
	}
	ctx := test.GetDeployContext(nil, []runtime.Object{dwSA})
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc"}

	containerBuildReconciler := NewContainerBuildReconciler()

	done, err := containerBuildReconciler.syncRBAC(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetUserSccRbacResourcesName()}, &rbacv1.ClusterRole{}))

	done, err = containerBuildReconciler.removeRBAC(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetUserSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
}

func TestSyncAndRemoveSCC(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc"}

	containerBuildReconciler := NewContainerBuildReconciler()

	done, err := containerBuildReconciler.syncSCC(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))

	done, err = containerBuildReconciler.removeSCC(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))
}

func TestShouldNotSyncSCCIfAlreadyExists(t *testing.T) {
	scc := &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc",
		},
	}

	ctx := test.GetDeployContext(nil, []runtime.Object{scc})
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc"}

	containerBuildReconciler := NewContainerBuildReconciler()

	done, err := containerBuildReconciler.syncSCC(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	scc = &securityv1.SecurityContextConstraints{}
	exists, err := deploy.GetClusterObject(ctx, "scc", scc)
	assert.True(t, exists)
	assert.Nil(t, err)

	// No labels must be added
	assert.True(t, scc.Labels[deploy.GetManagedByLabel()] == "")

	done, err = containerBuildReconciler.removeSCC(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	// Can't be removed
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))
}
