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

package containercapabilities

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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

	cheCtx := test.NewCtxBuilder().WithObjects(dwPod).Build()
	containerBuildReconciler := NewContainerCapabilitiesReconciler()

	test.EnsureReconcile(t, cheCtx, containerBuildReconciler.Reconcile)

	// Enable container capabilities
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = ptr.To(false)
	cheCtx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc-build"}

	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = ptr.To(false)
	cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration = &chev2.ContainerRunConfiguration{OpenShiftSecurityContextConstraint: "scc-run"}

	err := cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, cheCtx, containerBuildReconciler.Reconcile)

	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: "scc-build"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(cheCtx.CheCluster.Finalizers, containerBuildReconciler.containerBuildCapability.getFinalizer()))

	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: "scc-run"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.True(t, utils.Contains(cheCtx.CheCluster.Finalizers, containerBuildReconciler.containerRunCapability.getFinalizer()))

	crb := &rbacv1.ClusterRoleBinding{}
	_, err = cheCtx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleBindingName()},
		crb,
	)
	assert.NoError(t, err)
	assert.Equal(t, "devworkspace-controller", crb.Subjects[0].Namespace)

	crb = &rbacv1.ClusterRoleBinding{}
	_, err = cheCtx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleBindingName()},
		crb,
	)
	assert.NoError(t, err)
	assert.Equal(t, "devworkspace-controller", crb.Subjects[0].Namespace)

	// Disable Container capabilities
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = ptr.To(true)
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = ptr.To(true)

	err = cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, cheCtx, containerBuildReconciler.Reconcile)

	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: "scc-build"}, &securityv1.SecurityContextConstraints{}))
	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerBuildCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(cheCtx.CheCluster.Finalizers, containerBuildReconciler.containerBuildCapability.getFinalizer()))

	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: "scc-run"}, &securityv1.SecurityContextConstraints{}))
	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.getDWOClusterRoleBindingName()}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: containerBuildReconciler.containerRunCapability.GetUserRoleName()}, &rbacv1.ClusterRole{}))
	assert.False(t, utils.Contains(cheCtx.CheCluster.Finalizers, containerBuildReconciler.containerRunCapability.getFinalizer()))
}

func TestShouldUpdateManagedSCCOnReconcile(t *testing.T) {
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

	// Create an SCC managed by operator with outdated capabilities (missing CHOWN)
	sccRun := &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "scc-run",
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		AllowedCapabilities: []corev1.Capability{"SETUID", "SETGID"},
	}

	cheCtx := test.NewCtxBuilder().WithObjects(dwPod, sccRun).Build()

	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = ptr.To(true)
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = ptr.To(false)
	cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration = &chev2.ContainerRunConfiguration{OpenShiftSecurityContextConstraint: "scc-run"}
	err := cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
	assert.NoError(t, err)

	containerBuildReconciler := NewContainerCapabilitiesReconciler()
	test.EnsureReconcile(t, cheCtx, containerBuildReconciler.Reconcile)

	// Verify the SCC was updated with the new capabilities including CHOWN
	scc := &securityv1.SecurityContextConstraints{}
	exists, err := cheCtx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(cheCtx.Context, types.NamespacedName{Name: "scc-run"}, scc)
	assert.True(t, exists)
	assert.NoError(t, err)
	assert.Equal(t, []corev1.Capability{"SETUID", "SETGID", "CHOWN"}, scc.AllowedCapabilities)
	assert.Equal(t, securityv1.NamespaceLevelRequirePod, scc.UserNamespaceLevel)
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

	cheCtx := test.NewCtxBuilder().WithObjects(dwPod, sccBuild, sccRun).Build()
	cheCtx.DWONamespace = "devworkspace-controller"

	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = ptr.To(false)
	cheCtx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration = &chev2.ContainerBuildConfiguration{OpenShiftSecurityContextConstraint: "scc-build"}
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = ptr.To(false)
	cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration = &chev2.ContainerRunConfiguration{OpenShiftSecurityContextConstraint: "scc-run"}
	err := cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
	assert.NoError(t, err)

	containerBuildReconciler := NewContainerCapabilitiesReconciler()

	test.EnsureReconcile(t, cheCtx, containerBuildReconciler.Reconcile)

	scc := &securityv1.SecurityContextConstraints{}
	exists, err := cheCtx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(cheCtx.Context, types.NamespacedName{Name: "scc-build"}, scc)
	assert.True(t, exists)
	assert.Nil(t, err)
	assert.True(t, scc.Labels[deploy.GetManagedByLabel()] == "")

	scc = &securityv1.SecurityContextConstraints{}
	exists, err = cheCtx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(cheCtx.Context, types.NamespacedName{Name: "scc-run"}, scc)
	assert.True(t, exists)
	assert.Nil(t, err)
	assert.True(t, scc.Labels[deploy.GetManagedByLabel()] == "")

	// Disable Container capabilities
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = ptr.To(true)
	cheCtx.CheCluster.Spec.DevEnvironments.DisableContainerRunCapabilities = ptr.To(true)
	err = cheCtx.ClusterAPI.Client.Update(context.TODO(), cheCtx.CheCluster)
	assert.NoError(t, err)

	test.EnsureReconcile(t, cheCtx, containerBuildReconciler.Reconcile)

	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: "scc-build"}, &securityv1.SecurityContextConstraints{}))
	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: "scc-run"}, &securityv1.SecurityContextConstraints{}))
}
