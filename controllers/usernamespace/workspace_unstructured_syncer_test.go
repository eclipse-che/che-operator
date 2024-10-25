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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
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
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	deployContext := test.GetDeployContext(nil, []runtime.Object{
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
								"user":      "${PROJECT_REQUESTING_USER}",
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

	// Sync Template
	err := workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check LimitRange in a user namespace is created
	lr := &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypeContainer, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, "user", lr.Labels["user"])
	assert.Equal(t, userNamespace, lr.Labels["namespace"])

	// Update src Template
	template := &templatev1.Template{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInCheNs, template)
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
	err = workspaceConfigReconciler.client.Update(context.TODO(), template)
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination LimitRange is updated
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])

	// Update dst LimitRange
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	lr.Spec.Limits[0].Type = corev1.LimitTypePersistentVolumeClaim
	err = workspaceConfigReconciler.client.Update(context.TODO(), lr)
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination LimitRange is reverted
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])

	// Update dst LimitRange in the way that it won't be reverted
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	lr.Annotations = map[string]string{"new-annotation": "new-test"}
	utils.AddMap(lr.Labels, map[string]string{"new-label": "new-test"})
	err = workspaceConfigReconciler.client.Update(context.TODO(), lr)
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination ConfigMap is not reverted
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
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
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 2, v1LimitRangeGKV)

	// Check that destination LimitRange is reverted
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.Nil(t, err)
	assert.Equal(t, corev1.LimitTypePod, lr.Spec.Limits[0].Type)
	assert.Equal(t, constants.WorkspacesConfig, lr.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, lr.Labels[constants.KubernetesPartOfLabelKey])

	// Delete src Template
	err = deploy.DeleteIgnoreIfNotFound(context.TODO(), deployContext.ClusterAPI.Client, objectKeyInCheNs, &templatev1.Template{})
	assert.Nil(t, err)

	// Sync Template
	err = workspaceConfigReconciler.syncWorkspace(context.TODO(), userNamespace)
	assert.Nil(t, err)
	assertSyncConfig(t, workspaceConfigReconciler, 0, v1LimitRangeGKV)

	// Check that destination LimitRange in a user namespace is deleted
	lr = &corev1.LimitRange{}
	err = workspaceConfigReconciler.client.Get(context.TODO(), objectKeyInUserNs, lr)
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))
}
