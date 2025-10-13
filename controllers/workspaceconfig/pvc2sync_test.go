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

package workspace_config

import (
	"context"
	"sync"
	"testing"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	"k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncPVC(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}).Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Scheme,
		&namespacecache.NamespaceCache{
			Client: deployContext.ClusterAPI.Client,
			KnownNamespaces: map[string]namespacecache.NamespaceInfo{
				userNamespace: {
					IsWorkspaceNamespace: true,
					Username:             "user",
					CheCluster:           &types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"},
				},
			},
			Lock: sync.Mutex{},
		})

	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Sync PVC to a user namespace
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check if PVC in a user namespace is created
	pvc := &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, pvc.Labels[constants.KubernetesPartOfLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("1Gi")))

	// Update src PVC
	pvc = &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, pvc)
	assert.Nil(t, err)
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), pvc)

	// Sync PVC
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check that destination PVC is not updated
	pvc = &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, pvc.Labels[constants.KubernetesPartOfLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("1Gi")))

	// Delete dst PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInUserNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check if PVC in a user namespace is created again
	pvc = &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, pvc.Labels[constants.KubernetesPartOfLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("2Gi")))

	// Delete src PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Check that destination PersistentVolumeClaim in a user namespace is NOT deleted
	pvc = &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
}

func TestSyncPVCShouldRetainIfAnnotationSetTrue(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: eclipseCheNamespace,
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
				Annotations: map[string]string{
					syncRetainOnDeleteAnnotation: "true",
				},
			},
		}).Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Scheme,
		&namespacecache.NamespaceCache{
			Client: deployContext.ClusterAPI.Client,
			KnownNamespaces: map[string]namespacecache.NamespaceInfo{
				userNamespace: {
					IsWorkspaceNamespace: true,
					Username:             "user",
					CheCluster:           &types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"},
				},
			},
			Lock: sync.Mutex{},
		})

	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Sync PVC to a user namespace
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check if PVC in a user namespace is created
	pvc := &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, "true", pvc.Annotations[syncRetainOnDeleteAnnotation])

	// Delete src PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Check that destination PVC in a user namespace is NOT deleted
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, &corev1.PersistentVolumeClaim{})
	assert.NoError(t, err)
}

func TestSyncPVCShouldNotRetainIfAnnotationSetFalse(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: eclipseCheNamespace,
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
				Annotations: map[string]string{
					syncRetainOnDeleteAnnotation: "false",
				},
			},
		}).Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Scheme,
		&namespacecache.NamespaceCache{
			Client: deployContext.ClusterAPI.Client,
			KnownNamespaces: map[string]namespacecache.NamespaceInfo{
				userNamespace: {
					IsWorkspaceNamespace: true,
					Username:             "user",
					CheCluster:           &types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"},
				},
			},
			Lock: sync.Mutex{},
		})

	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Sync PVC to a user namespace
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check if PVC in a user namespace is created
	pvc := &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, "false", pvc.Annotations[syncRetainOnDeleteAnnotation])

	// Delete src PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Check that destination PVC in a user namespace is deleted
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, &corev1.PersistentVolumeClaim{})
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}
