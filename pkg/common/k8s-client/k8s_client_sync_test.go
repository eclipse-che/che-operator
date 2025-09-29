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
	"reflect"
	"testing"

	testclient "github.com/eclipse-che/che-operator/pkg/common/test/test-client"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	diffs = cmp.Options{
		cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
		cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
			return reflect.DeepEqual(x.Labels, y.Labels)
		}),
	}
)

func TestSync(t *testing.T) {
	fakeClient, _, scheme := testclient.GetTestClients()
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

	err := cli.Sync(context.TODO(), cm, &SyncOptions{DiffOpts: diffs})

	assert.NoError(t, err)
	assert.Equal(t, "ConfigMap", cm.Kind)
	assert.Equal(t, "v1", cm.APIVersion)

	newCm := &corev1.ConfigMap{
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

	err = cli.Sync(context.TODO(), newCm, &SyncOptions{DiffOpts: diffs})

	assert.NoError(t, err)
	assert.Equal(t, "ConfigMap", newCm.Kind)
	assert.Equal(t, "v1", newCm.APIVersion)

	newCm = &corev1.ConfigMap{
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

	err = cli.Sync(context.TODO(), newCm, &SyncOptions{DiffOpts: diffs})

	assert.NoError(t, err)
	assert.Equal(t, "ConfigMap", newCm.Kind)
	assert.Equal(t, "v1", newCm.APIVersion)
}

func TestSyncAndMergeLabelsAnnotations(t *testing.T) {
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
	cli := NewK8sClient(fakeClient, scheme)

	err := cli.Sync(
		context.TODO(),
		&corev1.ConfigMap{
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
		},
		&SyncOptions{MergeLabels: true, MergeAnnotations: true},
	)

	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = fakeClient.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "test"}, cm)

	assert.Equal(t, "value_1", cm.Labels["label_1"])
	assert.Equal(t, "cluster_value_2", cm.Labels["label_2"])
	assert.Equal(t, "value_3", cm.Labels["label_3"])
	assert.Equal(t, "value_1", cm.Annotations["annotation_1"])
	assert.Equal(t, "cluster_value_2", cm.Annotations["annotation_2"])
	assert.Equal(t, "value_3", cm.Annotations["annotation_3"])
}
