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
	"fmt"
	"testing"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreate(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Data: map[string]string{
				"key": "new-value",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "user-che",
			},
			Data: map[string]string{
				"key": "old_value",
			},
		},
	).Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	err := workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	dstCm := &corev1.ConfigMap{}
	exists, err := deploy.Get(ctx, types.NamespacedName{Namespace: "user-che", Name: "test"}, dstCm)

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 1, len(dstCm.Data))
	assert.Equal(t, "new-value", dstCm.Data["key"])
}

func TestUpdate(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient),
	)

	err := ctx.ClusterAPI.Client.Create(
		context.TODO(),
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Data: map[string]string{
				"key_1": "value_1",
			},
		})

	assert.NoError(t, err)

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	dstCm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "user-che"}, dstCm)

	assert.NoError(t, err)

	srcCm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, srcCm)

	assert.NoError(t, err)
	assert.True(t, cmp.Equal(dstCm, srcCm, diffs.ConfigMap([]string{constants.KubernetesPartOfLabelKey, constants.KubernetesComponentLabelKey}, nil)))

	// update source and destination config maps

	dstCm.Data["key_1"] = "new_dst_value_1"
	dstCm.Data["key_2"] = "new_dst_value_2"
	dstCm.Labels["label_1"] = "new_dst_value_1"
	dstCm.Labels["label_2"] = "new_dst_value_2"
	dstCm.Annotations = map[string]string{}
	dstCm.Annotations["annotation_1"] = "new_dst_value_1"
	dstCm.Annotations["annotation_2"] = "new_dst_value_2"
	err = ctx.ClusterAPI.Client.Update(context.TODO(), dstCm)

	assert.NoError(t, err)

	srcCm.Data["key_1"] = "new_src_value_1"
	srcCm.Data["key_3"] = "new_src_value_3"
	srcCm.Labels["label_1"] = "new_src_value_1"
	srcCm.Labels["label_3"] = "new_src_value_3"
	srcCm.Annotations = map[string]string{}
	srcCm.Annotations["annotation_1"] = "new_src_value_1"
	srcCm.Annotations["annotation_3"] = "new_src_value_3"

	err = ctx.ClusterAPI.Client.Update(context.TODO(), srcCm)

	assert.NoError(t, err)

	// check again

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	dstCm = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "user-che"}, dstCm)

	assert.NoError(t, err)

	srcCm = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, srcCm)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(dstCm.Data))
	assert.Equal(t, "new_src_value_1", dstCm.Data["key_1"])
	assert.Equal(t, "new_src_value_3", dstCm.Data["key_3"])
	assert.Equal(t, "new_src_value_1", dstCm.Labels["label_1"])
	assert.Equal(t, "new_dst_value_2", dstCm.Labels["label_2"])
	assert.Equal(t, "new_src_value_3", dstCm.Labels["label_3"])
	assert.Equal(t, "new_src_value_1", dstCm.Annotations["annotation_1"])
	assert.Equal(t, "new_dst_value_2", dstCm.Annotations["annotation_2"])
	assert.Equal(t, "new_src_value_3", dstCm.Annotations["annotation_3"])
}

func TestDeleteIfObjectIsObsolete(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	err := ctx.ClusterAPI.Client.Create(
		context.TODO(),
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test_1",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		})

	assert.NoError(t, err)

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	syncCMKey := types.NamespacedName{
		Name:      syncedWorkspacesConfig,
		Namespace: "user-che",
	}

	syncCM := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test_1", "eclipse-che")])
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test_1", "user-che")])
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)

	// add obsolete data to a sync config map

	syncCM.Data[buildKey(v1ConfigMapGKV, "test_2", "user-che")] = "1"
	syncCM.Data[buildKey(v1ConfigMapGKV, "test_2", "eclipse-che")] = "1"

	err = ctx.ClusterAPI.Client.Update(context.TODO(), syncCM)

	assert.NoError(t, err)

	err = ctx.ClusterAPI.Client.Create(
		context.TODO(),
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test_2",
				Namespace: "user-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		})

	// sync again to check that obsolete data will be removed

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	syncCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test_1", "eclipse-che")])
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test_1", "user-che")])
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)

	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test_2", Namespace: "user-che"}, &corev1.ConfigMap{})

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	// clean sync config map data

	syncCM.Data = map[string]string{}

	err = ctx.ClusterAPI.Client.Update(context.TODO(), syncCM)

	assert.NoError(t, err)

	// sync again to check that data will be restored

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	syncCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test_1", "eclipse-che")])
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test_1", "user-che")])
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)

	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test_1", Namespace: "user-che"}, &corev1.ConfigMap{})

	assert.NoError(t, err)
}

func TestBuildKey(t *testing.T) {
	type testCase struct {
		name      string
		namespace string
		gkv       schema.GroupVersionKind
	}

	testCases := []testCase{
		{
			name:      "test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		},
		{
			name:      "test.test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		},
		{
			name:      "test_test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		},
		{
			name:      "test-test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		},
		{
			name:      "test-test_test.test-test_test.test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		},

		{
			name:      "test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("Secret"),
		},
		{
			name:      "test",
			namespace: "eclipse-che",
			gkv:       corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim"),
		},
		{
			name:      "test",
			namespace: "eclipse-che",
			gkv:       rbacv1.SchemeGroupVersion.WithKind("Role"),
		},
		{
			name:      "test",
			namespace: "eclipse-che",
			gkv:       rbacv1.SchemeGroupVersion.WithKind("ClusterRole"),
		},
		{
			name:      "test.test",
			namespace: "eclipse-che",
			gkv:       rbacv1.SchemeGroupVersion.WithKind("ClusterRole"),
		},
		{
			name:      "test_test",
			namespace: "eclipse-che",
			gkv:       rbacv1.SchemeGroupVersion.WithKind("ClusterRole"),
		},
		{
			name:      "test-test",
			namespace: "eclipse-che",
			gkv:       rbacv1.SchemeGroupVersion.WithKind("ClusterRole"),
		},
		{
			name:      "test-test_test.test-test_test.test",
			namespace: "eclipse-che",
			gkv:       rbacv1.SchemeGroupVersion.WithKind("ClusterRole"),
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("case #%d", i), func(t *testing.T) {
			key := buildKey(testCase.gkv, testCase.name, testCase.namespace)

			assert.Equal(t, testCase.name, getNameItem(key))
			assert.Equal(t, testCase.namespace, getNamespaceItem(key))
			assert.Equal(t, testCase.gkv, item2gkv(getGkvItem(key)))
		})
	}
}

func TestSyncConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	syncCMKey := types.NamespacedName{
		Name:      syncedWorkspacesConfig,
		Namespace: "user-che",
	}

	// Sync config map should not exist
	syncCM := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	// Sync config map should exist
	syncCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Empty(t, syncCM.Data)
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)

	// sync some object and check sync config map

	err = ctx.ClusterAPI.Client.Create(
		context.TODO(),
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Data: map[string]string{
				"key": "value",
			},
		})

	assert.NoError(t, err)

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	// Sync config map should exist and contains synced object revision

	syncCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test", "eclipse-che")])
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test", "user-che")])
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)

	// Sync one more time, nothing should change

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	syncCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test", "eclipse-che")])
	assert.Equal(t, "1", syncCM.Data[buildKey(v1ConfigMapGKV, "test", "user-che")])
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)

	// delete some object and check sync config map

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)

	assert.NoError(t, err)

	err = ctx.ClusterAPI.Client.Delete(context.TODO(), cm)
	assert.NoError(t, err)

	err = workspaceConfigReconciler.syncNamespace(
		context.TODO(),
		"eclipse-che",
		"user-che",
	)

	assert.NoError(t, err)

	syncCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), syncCMKey, syncCM)

	assert.NoError(t, err)
	assert.Empty(t, syncCM.Data)
	assert.Equal(t,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
			constants.KubernetesManagedByLabelKey: deploy.GetManagedByLabel(),
		},
		syncCM.Labels)
}
