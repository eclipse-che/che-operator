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
)

func TestMergeLabelsAnnotationsFromClusterObject(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients(
		&corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
				Labels: map[string]string{
					"label_1": "cluster_value_1",
					"label_2": "cluster_value_2",
				},
				Annotations: map[string]string{
					"annotation_1": "cluster_value_1",
					"annotation_2": "cluster_value_2",
				},
			},
		})

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
			Labels: map[string]string{
				"label_1": "value_1",
				"label_3": "value_3",
			},
			Annotations: map[string]string{
				"annotation_1": "value_1",
				"annotation_3": "value_3",
			},
		},
	}

	err := MergeLabelsAnnotationsFromClusterObject(context.TODO(), scheme, fakeClient, cm)

	assert.NoError(t, err)
	assert.Equal(t, "value_1", cm.Labels["label_1"])
	assert.Equal(t, "cluster_value_2", cm.Labels["label_2"])
	assert.Equal(t, "value_3", cm.Labels["label_3"])
	assert.Equal(t, "value_1", cm.Annotations["annotation_1"])
	assert.Equal(t, "cluster_value_2", cm.Annotations["annotation_2"])
	assert.Equal(t, "value_3", cm.Annotations["annotation_3"])
}
