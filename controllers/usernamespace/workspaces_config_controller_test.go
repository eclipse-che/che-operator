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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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
	deployContext := test.GetDeployContext(nil, []runtime.Object{
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
				},
			},
			Data: map[string]string{
				"a": "b",
			},
			BinaryData: map[string][]byte{
				"c": []byte("d"),
			},
			Immutable: pointer.Bool(false),
		}})

	workspaceConfigReconciler := NewWorkspacesConfigReconciler(
		deployContext.ClusterAPI.Client,
		deployContext.ClusterAPI.NonCachingClient,
		deployContext.ClusterAPI.Scheme,
		NewNamespaceCache(deployContext.ClusterAPI.NonCachingClient))

	// Sync ConfigMap
	err := workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.ConfigMap{})

	// Check ConfigMap in a user namespace
	cm := &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "b", cm.Data["a"])
	assert.Equal(t, []byte("d"), cm.BinaryData["c"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update src ConfigMap
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, cm)
	assert.Nil(t, err)

	cm.Data["a"] = "c"
	cm.Annotations = map[string]string{
		"test": "test",
	}
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.ConfigMap{})

	// Check that destination ConfigMap in a user namespace is updated as well
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, []byte("d"), cm.BinaryData["c"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "test", cm.Annotations["test"])

	// Update dst ConfigMap in a user namespace
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)

	cm.Data["a"] = "new-c"
	cm.Annotations = map[string]string{
		"test": "new-test",
	}
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), cm)
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.ConfigMap{})

	// Check that destination ConfigMap in a user namespace is reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, []byte("d"), cm.BinaryData["c"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "test", cm.Annotations["test"])

	// Delete dst ConfigMap in a user namespace
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.NonCachingClient, objectKeyInUserNs, &corev1.ConfigMap{})
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	// Check that destination ConfigMap in a user namespace is reverted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.Nil(t, err)
	assert.Equal(t, "c", cm.Data["a"])
	assert.Equal(t, []byte("d"), cm.BinaryData["c"])
	assert.Equal(t, false, *cm.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, cm.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/watch-configmap"])
	assert.Equal(t, "true", cm.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "test", cm.Annotations["test"])

	// Delete src ConfigMap
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.NonCachingClient, objectKeyInCheNs, &corev1.ConfigMap{})
	assert.Nil(t, err)

	// Sync ConfigMap
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 0, &corev1.ConfigMap{})

	// Check that destination ConfigMap in a user namespace is deleted
	cm = &corev1.ConfigMap{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, cm)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}

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
	err := workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.Secret{})

	// Check Secret
	secret := &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "b", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])

	// Update src Secret
	secret = &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, secret)
	assert.Nil(t, err)

	secret.StringData["a"] = "c"
	secret.Annotations = map[string]string{
		"test": "test",
	}
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), secret)
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.Secret{})

	// Check that destination Secret is updated
	secret = &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "c", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "test", secret.Annotations["test"])

	// Update dst Secret
	secret = &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)

	secret.StringData["a"] = "new-c"
	secret.Annotations = map[string]string{
		"test": "new-test",
	}
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), secret)
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.Secret{})

	// Check that destination Secret is reverted
	secret = &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.Nil(t, err)
	assert.Equal(t, "c", secret.StringData["a"])
	assert.Equal(t, []byte("d"), secret.Data["c"])
	assert.Equal(t, false, *secret.Immutable)
	assert.Equal(t, constants.WorkspacesConfig, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/watch-secret"])
	assert.Equal(t, "true", secret.Labels["controller.devfile.io/mount-to-devworkspace"])
	assert.Equal(t, "test", secret.Annotations["test"])

	// Delete src Secret
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.NonCachingClient, objectKeyInCheNs, &corev1.Secret{})
	assert.Nil(t, err)

	// Sync Secret
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 0, &corev1.Secret{})

	// Check that destination Secret in a user namespace is deleted
	secret = &corev1.Secret{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, secret)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}

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
		deployContext.ClusterAPI.NonCachingClient,
		deployContext.ClusterAPI.Scheme,
		NewNamespaceCache(deployContext.ClusterAPI.NonCachingClient))

	validateSyncConfig(t, deployContext, 0, &corev1.PersistentVolumeClaim{})

	// Sync PVC
	err := workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 2, &corev1.PersistentVolumeClaim{})

	// Check PVC in a user namespace
	pvc := &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, pvc.Labels[constants.KubernetesComponentLabelKey])
	assert.True(t, pvc.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("1Gi")))

	// Delete src PVC
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.NonCachingClient, objectKeyInCheNs, &corev1.PersistentVolumeClaim{})
	assert.Nil(t, err)

	// Sync PVC
	err = workspaceConfigReconciler.syncWorkspacesConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)

	validateSyncConfig(t, deployContext, 0, &corev1.PersistentVolumeClaim{})

	// Check that destination PersistentVolumeClaim in a user namespace is deleted
	pvc = &corev1.PersistentVolumeClaim{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, pvc)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func validateSyncConfig(t *testing.T, deployContext *chetypes.DeployContext, expectedNumberOfRecords int, blueprint client.Object) {
	cm, err := getSyncConfig(context.TODO(), userNamespace, deployContext)
	assert.Nil(t, err)
	assert.Equal(t, expectedNumberOfRecords, len(cm.Data))
	if expectedNumberOfRecords == 2 {
		assert.Contains(t, cm.Data, computeObjectKey(deploy.GetObjectType(blueprint), objectName, userNamespace))
		assert.Contains(t, cm.Data, computeObjectKey(deploy.GetObjectType(blueprint), objectName, eclipseCheNamespace))
	}
}
