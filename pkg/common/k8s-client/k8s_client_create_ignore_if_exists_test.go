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
	"context"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCreate(t *testing.T) {
	cmExpected := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects().Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	done, err := k8sClient.Create(cmExpected, nil)

	assert.NoError(t, err)
	assert.True(t, done)

	cmActual := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cmActual)

	assert.NoError(t, err)
	assert.Equal(t, cmActual.Data, cmExpected.Data)
}

func TestCreateWithOwner(t *testing.T) {
	cmExpected := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-owner",
				Namespace: "eclipse-che",
			},
			Data: map[string]string{
				"key": "value",
			},
		},
	).Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	cmOwner := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "cm-owner", Namespace: "eclipse-che"}, cmOwner)

	assert.NoError(t, err)

	done, err := k8sClient.Create(cmExpected, cmOwner)

	assert.NoError(t, err)
	assert.True(t, done)

	cmActual := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cmActual)

	assert.NoError(t, err)
	assert.Equal(t, cmActual.Data, cmExpected.Data)
	assert.Equal(t, cmActual.OwnerReferences[0].Name, cmOwner.Name)
	assert.Equal(t, cmActual.OwnerReferences[0].Kind, "ConfigMap")
	assert.Equal(t, cmActual.OwnerReferences[0].APIVersion, "v1")
}

func TestCreateAlreadyExistedObject(t *testing.T) {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(cm).Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	done, err := k8sClient.Create(cm, nil)

	assert.Error(t, err)
	assert.False(t, done)
}
