//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeleteAllOf(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-1",
				Namespace: "eclipse-che",
				Labels:    map[string]string{"app": "test"},
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-2",
				Namespace: "eclipse-che",
				Labels:    map[string]string{"app": "test"},
			},
		},
	)
	cli := NewK8sClient(fakeClient, scheme)

	err := cli.DeleteAllOf(
		context.TODO(),
		&corev1.ConfigMap{},
		client.InNamespace("eclipse-che"),
		client.MatchingLabels{"app": "test"},
	)

	assert.NoError(t, err)

	list := &corev1.ConfigMapList{}
	err = fakeClient.List(
		context.TODO(),
		list,
		client.InNamespace("eclipse-che"),
		client.MatchingLabels{"app": "test"},
	)

	assert.NoError(t, err)
	assert.Empty(t, list.Items)
}

func TestDeleteAllOfInNamespace(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-1",
				Namespace: "eclipse-che",
			},
		},
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-2",
				Namespace: "other-ns",
			},
		},
	)
	cli := NewK8sClient(fakeClient, scheme)

	err := cli.DeleteAllOf(
		context.TODO(),
		&corev1.ConfigMap{},
		client.InNamespace("eclipse-che"),
	)

	assert.NoError(t, err)

	list := &corev1.ConfigMapList{}
	err = fakeClient.List(context.TODO(), list, client.InNamespace("eclipse-che"))

	assert.NoError(t, err)
	assert.Empty(t, list.Items)

	err = fakeClient.List(context.TODO(), list, client.InNamespace("other-ns"))

	assert.NoError(t, err)
	assert.Equal(t, 1, len(list.Items))
	assert.Equal(t, "cm-2", list.Items[0].Name)
}

func TestDeleteAllOfNoMatchingObjects(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cm-1",
				Namespace: "eclipse-che",
				Labels:    map[string]string{"app": "other"},
			},
		},
	)
	cli := NewK8sClient(fakeClient, scheme)

	err := cli.DeleteAllOf(
		context.TODO(),
		&corev1.ConfigMap{},
		client.InNamespace("eclipse-che"),
		client.MatchingLabels{"app": "test"},
	)

	assert.NoError(t, err)

	list := &corev1.ConfigMapList{}
	err = fakeClient.List(context.TODO(), list, client.InNamespace("eclipse-che"))

	assert.NoError(t, err)
	assert.Equal(t, 1, len(list.Items))
}

func TestDeleteAllOfEmptyNamespace(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients()
	cli := NewK8sClient(fakeClient, scheme)

	err := cli.DeleteAllOf(
		context.TODO(),
		&corev1.ConfigMap{},
		client.InNamespace("eclipse-che"),
	)

	assert.NoError(t, err)
}
