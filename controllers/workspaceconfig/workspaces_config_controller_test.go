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

func TestRecreateObjectIfAlreadyExists(t *testing.T) {
	// Actual object in a user namespace
	srcObject := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "user-che",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	// Expected object in a user namespace
	dstObject := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "user-che",
		},
		Data: map[string]string{
			"new-key": "new-value",
		},
	}

	ctx := test.NewCtxBuilder().Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	syncContext := &syncContext{
		dstNamespace: "user-che",
		srcNamespace: "eclipse-che",
		object2Sync:  &configMap2Sync{cm: srcObject},
		syncConfig:   map[string]string{},
	}

	err := workspaceConfigReconciler.doCreateObject(syncContext, dstObject)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	exists, err := deploy.Get(ctx, types.NamespacedName{Namespace: "user-che", Name: "test"}, cm)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, 1, len(cm.Data))
	assert.Equal(t, "new-value", cm.Data["new-key"])
}

func TestDeleteIfObjectIsObsolete(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test_1",
			Namespace: "user-che",
		},
	}).Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	test1CMInUserNS := buildKey(v1ConfigMapGKV, "test_1", "user-che")
	test2CMInUserNS := buildKey(v1ConfigMapGKV, "test_2", "user-che")
	test1CMInCheNS := buildKey(v1ConfigMapGKV, "test_1", "eclipse-che")
	test2CMInCheNS := buildKey(v1ConfigMapGKV, "test_2", "eclipse-che")

	syncConfig := map[string]string{
		test1CMInUserNS: "1",
		test1CMInCheNS:  "1",
		test2CMInUserNS: "1",
		test2CMInCheNS:  "1",
	}

	exists, err := deploy.Get(ctx, types.NamespacedName{Namespace: "user-che", Name: "test_1"}, &corev1.ConfigMap{})
	assert.NoError(t, err)
	assert.True(t, exists)

	// Should delete, since the object from source namespace is obsolete
	err = workspaceConfigReconciler.deleteIfObjectIsObsolete(
		test1CMInCheNS,
		context.TODO(),
		"eclipse-che",
		"user-che",
		syncConfig,
		map[string]bool{},
	)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(syncConfig))
	assert.Contains(t, syncConfig, test2CMInUserNS)
	assert.Contains(t, syncConfig, test2CMInCheNS)

	exists, err = deploy.Get(ctx, types.NamespacedName{Namespace: "user-che", Name: "test_1"}, &corev1.ConfigMap{})
	assert.NoError(t, err)
	assert.False(t, exists)

	// Should NOT delete, since the object from a user destination namespace
	err = workspaceConfigReconciler.deleteIfObjectIsObsolete(
		test2CMInUserNS,
		context.TODO(),
		"eclipse-che",
		"user-che",
		syncConfig,
		map[string]bool{},
	)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(syncConfig))
	assert.Contains(t, syncConfig, test2CMInUserNS)
	assert.Contains(t, syncConfig, test2CMInCheNS)
}

func TestGetEmptySyncConfig(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		namespacecache.NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	cm, err := workspaceConfigReconciler.getSyncConfig(context.TODO(), "eclipse-che")
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	assert.Empty(t, cm.Data)
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, deploy.GetManagedByLabel(), cm.Labels[constants.KubernetesManagedByLabelKey])
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
