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
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleteIfObjectIsObsolete(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test_1",
				Namespace: "user-che",
			},
		},
	})

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

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
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		ctx.ClusterAPI.Client,
		ctx.ClusterAPI.Scheme,
		NewNamespaceCache(ctx.ClusterAPI.NonCachingClient))

	cm, err := workspaceConfigReconciler.getSyncConfig(context.TODO(), "eclipse-che")
	assert.NoError(t, err)
	assert.NotNil(t, cm)
	assert.Empty(t, cm.Data)
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, deploy.GetManagedByLabel(), cm.Labels[constants.KubernetesManagedByLabelKey])
}

func TestIsDiff(t *testing.T) {
	src := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "eclipse-che",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	dst := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "eclipse-che",
			Labels:      map[string]string{"a": "b"},
			Annotations: map[string]string{"c": "d"},
		},
	}

	changed := isDiff(src, dst)
	assert.False(t, changed)
}
