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

	testclient "github.com/eclipse-che/che-operator/pkg/common/test/test-client"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	fakeClient, _, scheme := testclient.GetTestClients()
	cli := NewK8sClient(fakeClient, scheme)

	err := cli.Create(context.TODO(), cmExpected, nil)

	assert.NoError(t, err)
	assert.Equal(t, "ConfigMap", cmExpected.Kind)
	assert.Equal(t, "v1", cmExpected.APIVersion)

	cmActual := &corev1.ConfigMap{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cmActual)

	assert.NoError(t, err)
	cmp.Diff(cmActual, cmExpected, cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	})
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
	cmOwner := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-owner",
			Namespace: "eclipse-che",
		},
	}

	fakeClient, _, scheme := testclient.GetTestClients(cmOwner)
	cli := NewK8sClient(fakeClient, scheme)

	cmOwner = &corev1.ConfigMap{}
	err := fakeClient.Get(context.TODO(), types.NamespacedName{Name: "cm-owner", Namespace: "eclipse-che"}, cmOwner)

	assert.NoError(t, err)

	err = cli.Create(context.TODO(), cmExpected, cmOwner)

	assert.NoError(t, err)
	assert.Equal(t, "ConfigMap", cmExpected.Kind)
	assert.Equal(t, "v1", cmExpected.APIVersion)

	cmActual := &corev1.ConfigMap{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cmActual)

	assert.NoError(t, err)
	assert.Equal(t, cmActual.OwnerReferences[0].Name, cmOwner.Name)
	assert.Equal(t, cmActual.OwnerReferences[0].Kind, "ConfigMap")
	assert.Equal(t, cmActual.OwnerReferences[0].APIVersion, "v1")
	cmp.Diff(cmActual, cmExpected, cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	})
}

func TestCreateAlreadyExistedObject(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&corev1.ConfigMap{
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
		},
	)
	cli := NewK8sClient(fakeClient, scheme)

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
	}
	err := cli.Create(context.TODO(), cm, nil)

	assert.Error(t, err)
	assert.Equal(t, "ConfigMap", cm.Kind)
	assert.Equal(t, "v1", cm.APIVersion)
	assert.True(t, errors.IsAlreadyExists(err))
}
