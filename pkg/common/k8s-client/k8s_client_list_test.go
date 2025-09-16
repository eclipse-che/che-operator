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
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestList(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm1",
				Namespace: "eclipse-che",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm2",
				Namespace: "eclipse-che",
			},
		}).Build()

	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)
	objs, err := k8sClient.List(&corev1.ConfigMapList{})

	assert.NoError(t, err)
	assert.Equal(t, 2, len(objs))

	for _, obj := range objs {
		_, ok := obj.(*corev1.ConfigMap)
		assert.Equal(t, "ConfigMap", obj.GetObjectKind().GroupVersionKind().Kind)
		assert.Equal(t, "v1", obj.GetObjectKind().GroupVersionKind().Version)
		assert.Equal(t, "", obj.GetObjectKind().GroupVersionKind().Group)
		assert.True(t, ok)
	}
}

//func TestGetExistedObject(t *testing.T) {
//	ctx := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
//		TypeMeta: metav1.TypeMeta{
//			Kind:       "ConfigMap",
//			APIVersion: "v1",
//		},
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      "test",
//			Namespace: "eclipse-che",
//		},
//	}).Build()
//	syncer := ObjSyncer{
//		cli:    ctx.ClusterAPI.Client,
//		scheme: ctx.ClusterAPI.Scheme,
//	}
//
//	cm := &corev1.ConfigMap{}
//	exists, err := syncer.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)
//	assert.NoError(t, err)
//	assert.True(t, exists)
//}
//
//func TestGetNotExistedObject(t *testing.T) {
//	ctx := test.NewCtxBuilder().Build()
//	syncer := ObjSyncer{
//		cli:    ctx.ClusterAPI.Client,
//		scheme: ctx.ClusterAPI.Scheme,
//	}
//
//	cm := &corev1.ConfigMap{}
//	exists, err := syncer.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)
//	assert.NoError(t, err)
//	assert.False(t, exists)
//}
