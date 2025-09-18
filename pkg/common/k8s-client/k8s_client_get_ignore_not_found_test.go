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
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestGetExistedObject(t *testing.T) {
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
		})
	cli := NewK8sClient(fakeClient, scheme)

	cm := &corev1.ConfigMap{}
	exists, err := cli.GetIgnoreNotFound(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "v1", cm.APIVersion)
	assert.Equal(t, "ConfigMap", cm.Kind)
}

func TestGetNotExistedObject(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients()
	cli := NewK8sClient(fakeClient, scheme)

	cm := &corev1.ConfigMap{}
	exists, err := cli.GetIgnoreNotFound(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, cm)

	assert.NoError(t, err)
	assert.False(t, exists)
}
