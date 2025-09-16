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

package k8s_client

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetExistedObject(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
		}).Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	cm := &corev1.ConfigMap{}
	exists, err := k8sClient.Get(types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "v1", cm.APIVersion)
	assert.Equal(t, "ConfigMap", cm.Kind)
}

func TestGetNotExistedObject(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects().Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	cm := &corev1.ConfigMap{}
	exists, err := k8sClient.Get(types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)

	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestGetClusterScopedExistedObject(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(
		&consolev1.ConsoleLink{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConsoleLink",
				APIVersion: "console.openshift.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		},
	).Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	link := &consolev1.ConsoleLink{}
	exists, err := k8sClient.GetClusterScoped("test", link)

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "console.openshift.io/v1", link.APIVersion)
	assert.Equal(t, "ConsoleLink", link.Kind)
}

func TestGetClusterScopedNotExistedObject(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects().Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	link := &consolev1.ConsoleLink{}
	exists, err := k8sClient.GetClusterScoped("test", link)

	assert.NoError(t, err)
	assert.False(t, exists)
}
