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

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

const (
	eclipseCheNamespace = "eclipse-che"
	userNamespace       = "user-namespace"
	objectName          = "che-workspaces-config"
)

var (
	objectKeyInUserNs = types.NamespacedName{Name: objectName, Namespace: userNamespace}
	objectKeyInCheNs  = types.NamespacedName{Name: objectName, Namespace: eclipseCheNamespace}
)

func TestSyncConfigMap(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: "eclipse-che",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			},
			Annotations: map[string]string{},
		},
		Data: map[string]string{
			"a": "b",
		},
		Immutable: pointer.Bool(false),
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

	// Sync ConfigMap
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check ConfigMap in a user namespace is created
	cm := &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "b", cm.Data["a"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update src ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, cm)
	assert.Nil(t, err)
	cm.Data["a"] = "c"
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is updated
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update dst ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	cm.Data["a"] = "new-c"
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update dst ConfigMap in the way that it won't be reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	cm.Annotations = map[string]string{"new-annotation": "new-test"}
	utils.AddMap(cm.Labels, map[string]string{"new-label": "new-test"})
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is not reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "new-test", cm.Labels["new-label"])
	assert.Equal(t, "new-test", cm.Annotations["new-annotation"])

	// Delete dst ConfigMap
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInUserNs, &corev1.ConfigMap{})
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Delete src ConfigMap
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &corev1.ConfigMap{})
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1ConfigMapGKV)

	// Check that destination ConfigMap in a user namespace is deleted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestSyncConfigMapShouldMergeLabelsAndAnnotationsOnUpdate(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"label":                               "label-value",
				constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			},
			Annotations: map[string]string{
				"annotation": "annotation-value",
			},
		},
		Data: map[string]string{
			"a": "b",
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

	// Sync ConfigMap
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check ConfigMap in a user namespace is created
	cm := &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "label-value", cm.Labels["label"])
	assert.Equal(t, "annotation-value", cm.Annotations["annotation"])

	// Update labels and annotations on dst ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	utils.AddMap(cm.Labels, map[string]string{"new-label": "new-label-value"})
	utils.AddMap(cm.Annotations, map[string]string{"new-annotation": "new-annotation-value"})
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is not reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "label-value", cm.Labels["label"])
	assert.Equal(t, "new-label-value", cm.Labels["new-label"])
	assert.Equal(t, "annotation-value", cm.Annotations["annotation"])
	assert.Equal(t, "new-annotation-value", cm.Annotations["new-annotation"])

	// Update src ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, cm)
	assert.Nil(t, err)
	cm.Data["a"] = "c"
	utils.AddMap(cm.Labels, map[string]string{"label": "label-value-2"})
	utils.AddMap(cm.Annotations, map[string]string{"annotation": "annotation-value-2"})
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is updated but old labels and annotations are preserved
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "label-value-2", cm.Labels["label"])
	assert.Equal(t, "new-label-value", cm.Labels["new-label"])
	assert.Equal(t, "annotation-value-2", cm.Annotations["annotation"])
	assert.Equal(t, "new-annotation-value", cm.Annotations["new-annotation"])
}

func assertSyncConfig(t *testing.T, workspaceConfigReconciler *WorkspacesConfigReconciler, expectedNumberOfRecords int, gkv schema.GroupVersionKind) {
	cm, err := workspaceConfigReconciler.getSyncConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assert.Equal(t, expectedNumberOfRecords, len(cm.Data))
	if expectedNumberOfRecords == 2 {
		assert.Contains(t, cm.Data, buildKey(gkv, objectName, userNamespace))
		assert.Contains(t, cm.Data, buildKey(gkv, objectName, eclipseCheNamespace))
	}
}

func TestSyncConfigMapShouldRespectDWOLabels(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: "eclipse-che",
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey:          constants.CheEclipseOrg,
				constants.KubernetesComponentLabelKey:       constants.WorkspacesConfig,
				dwconstants.DevWorkspaceWatchConfigMapLabel: "false",
				dwconstants.DevWorkspaceMountLabel:          "false",
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

	// Sync ConfigMap
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check ConfigMap in a user namespace is created
	cm := &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "false", cm.Labels[dwconstants.DevWorkspaceWatchConfigMapLabel])
	assert.Equal(t, "false", cm.Labels[dwconstants.DevWorkspaceMountLabel])

	// Update DWO labels in dst ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	utils.AddMap(cm.Labels, map[string]string{
		dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
		dwconstants.DevWorkspaceMountLabel:          "true",
	})
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that dst ConfigMap is reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "false", cm.Labels[dwconstants.DevWorkspaceWatchConfigMapLabel])
	assert.Equal(t, "false", cm.Labels[dwconstants.DevWorkspaceMountLabel])

	// Update src ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, cm)
	assert.Nil(t, err)
	utils.AddMap(cm.Labels, map[string]string{
		dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
		dwconstants.DevWorkspaceMountLabel:          "true",
	})
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check that destination ConfigMap is updated
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", cm.Labels[dwconstants.DevWorkspaceWatchConfigMapLabel])
	assert.Equal(t, "true", cm.Labels[dwconstants.DevWorkspaceMountLabel])
}

func TestSyncConfigMapShouldRemoveSomeLabels(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
					"argocd.argoproj.io/instance":         "argocd",
					"argocd.argoproj.io/managed-by":       "argocd",
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

	// Sync ConfigMap
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1ConfigMapGKV)

	// Check ConfigMap in a user namespace is created
	cm := &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Empty(t, cm.Labels["argocd.argoproj.io/instance"])
	assert.Empty(t, cm.Labels["argocd.argoproj.io/managed-by"])
}
