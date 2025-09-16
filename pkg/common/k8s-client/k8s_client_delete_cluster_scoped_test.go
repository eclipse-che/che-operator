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
	"k8s.io/apimachinery/pkg/types"
)

func TestDelete(t *testing.T) {
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

	done, err := k8sClient.Delete(types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, &corev1.ConfigMap{})

	assert.NoError(t, err)
	assert.True(t, done)
}

func TestDeleteNotExistedObject(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects().Build()
	k8sClient := NewK8sClient(ctx.ClusterAPI.Client, ctx.ClusterAPI.Scheme)

	done, err := k8sClient.Delete(types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, &corev1.ConfigMap{})

	assert.NoError(t, err)
	assert.True(t, done)
}
