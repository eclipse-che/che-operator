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

	"k8s.io/apimachinery/pkg/types"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	templatev1 "github.com/openshift/api/template/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
)

var (
	v1LimitRangeGKV = corev1.SchemeGroupVersion.WithKind("LimitRange")
)

func TestSyncTemplateWithLimitRange(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&templatev1.Template{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Template",
				APIVersion: "template.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Objects: []runtime.RawExtension{
				{
					Object: &corev1.LimitRange{
						TypeMeta: metav1.TypeMeta{
							Kind:       "LimitRange",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: objectName,
							Labels: map[string]string{
								"user":      "${PROJECT_ADMIN_USER}",
								"namespace": "${PROJECT_NAME}",
							},
						},
						Spec: corev1.LimitRangeSpec{
							[]corev1.LimitRangeItem{
								{
									Type: corev1.LimitTypeContainer,
								},
							},
						},
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

	// Sync Template
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check LimitRange in a user namespace is created
	lr := &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypeContainer, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "user", lr.Labels["user"])
	assert.Equal(t, userNamespace, lr.Labels["namespace"])

	// Update src Template
	template := &templatev1.Template{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInCheNs, template)
	assert.Nil(t, err)
	template.Objects = []runtime.RawExtension{
		{
			Object: &corev1.LimitRange{
				TypeMeta: metav1.TypeMeta{
					Kind:       "LimitRange",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: objectName,
				},
				Spec: corev1.LimitRangeSpec{
					[]corev1.LimitRangeItem{
						{
							Type: corev1.LimitTypePod,
						},
					},
				},
			},
		},
	}
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), template)
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination LimitRange is updated
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])

	// Update dst LimitRange
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	lr.Spec.Limits[0].Type = corev1.LimitTypePersistentVolumeClaim
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), lr)
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination LimitRange is reverted
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])

	// Update dst LimitRange in the way that it won't be reverted
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	lr.Annotations = map[string]string{"new-annotation": "new-test"}
	utils.AddMap(lr.Labels, map[string]string{"new-label": "new-test"})
	err = deployContext.ClusterAPI.Client.Update(context.TODO(), lr)
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination ConfigMap is not reverted
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "new-test", lr.Labels["new-label"])
	assert.Equal(t, "new-test", lr.Annotations["new-annotation"])

	// Delete dst LimitRange
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInUserNs, &corev1.LimitRange{})
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination LimitRange is reverted
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])

	// Delete src Template
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &templatev1.Template{})
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1LimitRangeGKV)

	// Check that destination LimitRange in a user namespace is deleted
	lr = &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestSyncUnstructuredShouldRetainIfAnnotationSetTrue(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&templatev1.Template{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Template",
				APIVersion: "template.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Objects: []runtime.RawExtension{
				{
					Object: &corev1.LimitRange{
						TypeMeta: metav1.TypeMeta{
							Kind:       "LimitRange",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: objectName,
							Labels: map[string]string{
								"user":      "${PROJECT_ADMIN_USER}",
								"namespace": "${PROJECT_NAME}",
							},
							Annotations: map[string]string{
								syncRetainAnnotation: "true",
							},
						},
						Spec: corev1.LimitRangeSpec{
							[]corev1.LimitRangeItem{
								{
									Type: corev1.LimitTypeContainer,
								},
							},
						},
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

	// Sync Template
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check LimitRange in a user namespace is created
	lr := &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, "true", lr.Annotations[syncRetainAnnotation])

	// Delete src Template
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &templatev1.Template{})
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1LimitRangeGKV)

	// Check that destination LimitRange in a user namespace is NOT deleted
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, &corev1.LimitRange{})
	assert.NoError(t, err)
}

func TestSyncUnstructuredShouldNotRetainIfAnnotationSetFalse(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&templatev1.Template{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Template",
				APIVersion: "template.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Objects: []runtime.RawExtension{
				{
					Object: &corev1.LimitRange{
						TypeMeta: metav1.TypeMeta{
							Kind:       "LimitRange",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: objectName,
							Labels: map[string]string{
								"user":      "${PROJECT_ADMIN_USER}",
								"namespace": "${PROJECT_NAME}",
							},
							Annotations: map[string]string{
								syncRetainAnnotation: "false",
							},
						},
						Spec: corev1.LimitRangeSpec{
							[]corev1.LimitRangeItem{
								{
									Type: corev1.LimitTypeContainer,
								},
							},
						},
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

	// Sync Template
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check LimitRange in a user namespace is created
	lr := &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, "false", lr.Annotations[syncRetainAnnotation])

	// Delete src Template
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &templatev1.Template{})
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1LimitRangeGKV)

	// Check that destination LimitRange in a user namespace is deleted
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, &corev1.LimitRange{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestSyncUnstructuredShouldNotRetainIfAnnotationIsNotSet(t *testing.T) {
	deployContext := test.NewCtxBuilder().WithObjects(
		&templatev1.Template{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Template",
				APIVersion: "template.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      objectName,
				Namespace: "eclipse-che",
				Labels: map[string]string{
					constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
					constants.KubernetesComponentLabelKey: constants.WorkspacesConfig,
				},
			},
			Objects: []runtime.RawExtension{
				{
					Object: &corev1.LimitRange{
						TypeMeta: metav1.TypeMeta{
							Kind:       "LimitRange",
							APIVersion: "v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: objectName,
							Labels: map[string]string{
								"user":      "${PROJECT_ADMIN_USER}",
								"namespace": "${PROJECT_NAME}",
							},
						},
						Spec: corev1.LimitRangeSpec{
							[]corev1.LimitRangeItem{
								{
									Type: corev1.LimitTypeContainer,
								},
							},
						},
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

	// Sync Template
	err := workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check LimitRange in a user namespace is created
	lr := &corev1.LimitRange{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)

	// Delete src Template
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &templatev1.Template{})
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncNamespace(context.TODO(), eclipseCheNamespace, userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1LimitRangeGKV)

	// Check that destination LimitRange in a user namespace is deleted
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), objectKeyInUserNs, &corev1.LimitRange{})
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
