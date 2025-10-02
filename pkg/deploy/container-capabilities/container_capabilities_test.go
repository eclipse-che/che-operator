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

package containercapabilities

import (
	"context"
	"testing"

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
)

func TestContainerBuildReconciler(t *testing.T) {
	dwPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-controller",
			Namespace: "devworkspace-controller",
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

	// Enable container capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc-build"}

	ctx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = pointer.Bool(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration = &chev2.ContainerRunConfiguration{OpenShiftSecurityContextConstraint: "scc-run"}

	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc-build"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.containerBuildCapability.getFinalizer()))

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc-run"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.containerRunCapability.getFinalizer()))

	crb := &rbacv1.ClusterRoleBinding{}
	_, err = deploy.GetClusterObject(ctx, containerBuildReconciler.containerBuildCapability.getDWOClusterRoleBindingName(), crb)
	assert.NoError(t, err)
	assert.Equal(t, "devworkspace-controller", crb.Subjects[0].Namespace)

	crb = &rbacv1.ClusterRoleBinding{}
	_, err = deploy.GetClusterObject(ctx, containerBuildReconciler.containerRunCapability.getDWOClusterRoleBindingName(), crb)
	assert.NoError(t, err)
	assert.Equal(t, "devworkspace-controller", crb.Subjects[0].Namespace)

	// Disable Container capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(true)
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = pointer.Bool(true)

	err = ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc-build"}, &securityv1.SecurityContextConstraints{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.containerBuildCapability.getFinalizer()))

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc-run"}, &securityv1.SecurityContextConstraints{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, containerBuildReconciler.containerRunCapability.getFinalizer()))
}

func TestShouldNotSyncSCCIfAlreadyExists(t *testing.T) {
	dwPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "devworkspace-controller",
			Namespace: "devworkspace-controller",
			Labels: map[string]string{
				constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
				constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: constants.DevWorkspaceServiceAccountName,
		},
	}

	sccBuild := &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-build",
		},
	}
	sccRun := &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "scc-run",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(dwPod, sccBuild, sccRun).Build()

	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc-build"}
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = pointer.Bool(false)
	ctx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration = &chev2.ContainerRunConfiguration{OpenShiftSecurityContextConstraint: "scc-run"}
	err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)

	containerBuildReconciler := NewContainerBuildReconciler()

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	scc := &securityv1.SecurityContextConstraints{}
	exists, err := deploy.GetClusterObject(ctx, "scc-build", scc)
	assert.True(t, exists)
	assert.Nil(t, err)
	assert.True(t, scc.Labels[deploy.GetManagedByLabel()] == "")

	scc = &securityv1.SecurityContextConstraints{}
	exists, err = deploy.GetClusterObject(ctx, "scc-run", scc)
	assert.True(t, exists)
	assert.Nil(t, err)
	assert.True(t, scc.Labels[deploy.GetManagedByLabel()] == "")

	// Disable Container capabilities
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.Bool(true)
	ctx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = pointer.Bool(true)
	err = ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster)

	test.EnsureReconcile(t, ctx, containerBuildReconciler.Reconcile)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc-build"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "scc-run"}, &securityv1.SecurityContextConstraints{}))
}
