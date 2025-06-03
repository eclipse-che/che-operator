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

package sync

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetExistedObject(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
		},
	})
	syncer := ObjSyncer{
		cli:    ctx.ClusterAPI.Client,
		scheme: ctx.ClusterAPI.Scheme,
	}

	cm := &corev1.ConfigMap{}
	exists, err := syncer.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestGetNotExistedObject(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})
	syncer := ObjSyncer{
		cli:    ctx.ClusterAPI.Client,
		scheme: ctx.ClusterAPI.Scheme,
	}

	cm := &corev1.ConfigMap{}
	exists, err := syncer.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.False(t, exists)
}
