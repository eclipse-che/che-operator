//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"testing"
)

func TestContainerBuildReconciler(t *testing.T) {
	dwPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-controller",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: constants.DevWorkspaceServiceAccountName,
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(dwPod).Build()
	containerBuildReconciler := NewContainerBuildReconciler()

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	// Enable Container build capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc"}
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetUserSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.getFinalizerName()))

	// Disable Container build capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(true)
	err = ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc"}, &securityv1.SecurityContextConstraints{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetDevWorkspaceSccRbacResourcesName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: GetUserSccRbacResourcesName()}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.getFinalizerName()))
}

func TestSyncAndRemoveRBAC(t *testing.T) {
	dwPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-controller",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: constants.DevWorkspaceServiceAccountName,
		},
	}
	ctx := test.NewCtxBuilder().WithObjects(dwPod).Build()
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(false)
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
	ctx := test.NewCtxBuilder().Build()
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(false)
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

	ctx := test.NewCtxBuilder().WithObjects(scc).Build()
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
