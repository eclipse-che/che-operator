//
// Copyright (c) 2019-2024 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package usernamespace

import (
	"context"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/types"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestSyncPVC(t *testing.T) {
	deployContext := test.GetDeployContext(nil, []runtime.Object{
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
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}})

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.Scheme,
		&namespaceCache{
			client: deployContext.ClusterAPI.Client,
			knownNamespaces: map[string]namespaceInfo{
				userNamespace: {
					IsWorkspaceNamespace: true,
					Username:             "user",
					CheCluster:           &types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"},
				},
			},
			lock: sync.Mutex{},
		})

	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Sync PVC to a user namespace
	err := workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check if PVC in a user namespace is created
	pvc := &corev1.PersistentVolumeClaim{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, pvc.Labels[constants.KubernetesPartOfLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("1Gi")))

	// Update src PVC
	pvc = &corev1.PersistentVolumeClaim{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInCheNs, pvc)
	assert.Nil(t, err)
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse("2Gi")
	err = workspaceConfigReconciler.client.Update(context.TODO(), pvc)

	// Sync PVC
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check that destination PVC is not updated
	pvc = &corev1.PersistentVolumeClaim{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, pvc.Labels[constants.KubernetesPartOfLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("1Gi")))

	// Delete dst PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), workspaceConfigReconciler.client, objectKeyInUserNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1PvcGKV)

	// Check if PVC in a user namespace is created again
	pvc = &corev1.PersistentVolumeClaim{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, pvc.Labels[constants.KubernetesPartOfLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("2Gi")))

	// Delete src PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), workspaceConfigReconciler.client, objectKeyInCheNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1PvcGKV)

	// Check that destination PersistentVolumeClaim in a user namespace is deleted
	pvc = &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}
