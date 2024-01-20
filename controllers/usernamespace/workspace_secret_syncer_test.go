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

package usernamespace

import (
	"context"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/utils"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func TestSyncSecrets(t *testing.T) {
	deployContext := test.GetDeployContext(nil, []runtime.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
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
			StringData: map[string]string{
				"a": "b",
			},
			Data: map[string][]byte{
				"c": []byte("d"),
			},
			Immutable: pointer.Bool(false),
		}})

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.NonCachingClient,
		deployContext.ClusterAPI.Scheme,
		NewNamespaceCache(deployContext.ClusterAPI.NonCachingClient))

	// Sync Secret
	err := workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1SecretGKV)

	// Check Secret in a user namespace is created
	secret := &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "b", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update src Secret
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInCheNs, secret)
	assert.Nil(t, err)
	secret.StringData["a"] = "c"
	secret.Annotations = map[string]string{
		"test": "test",
	}
	err = workspaceConfigReconciler.client.Update(context.TODO(), secret)
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1SecretGKV)

	// Check that destination Secret is updated
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "c", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "test", secret.Annotations["test"])

	// Update dst Secret
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	secret.StringData["a"] = "new-c"
	err = workspaceConfigReconciler.client.Update(context.TODO(), secret)
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1SecretGKV)

	// Check that destination Secret is reverted
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "c", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update dst Secret in the way that it won't be reverted
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	utils.AddMap(secret.Annotations, map[string]string{"new-annotation": "new-test"})
	utils.AddMap(secret.Labels, map[string]string{"new-label": "new-test"})
	err = workspaceConfigReconciler.client.Update(context.TODO(), secret)
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1SecretGKV)

	// Check that destination Secret is not reverted
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "c", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "new-test", secret.Labels["new-label"])
	assert.Equal(t, "new-test", secret.Annotations["new-annotation"])

	// Delete dst Secret
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInUserNs, &corev1.Secret{})
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1SecretGKV)

	// Check that destination Secret is reverted
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "c", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Delete src Secret
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &corev1.Secret{})
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1SecretGKV)

	// Check that destination Secret in a user namespace is deleted
	secret = &corev1.Secret{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}
