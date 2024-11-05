//
// Copyright (c) 2019-2024 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package usernamespace

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsDiff(t *testing.T) {
	src := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "eclipse-che",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}

	dst := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "eclipse-che",
			Labels:      map[string]string{"a": "b"},
			Annotations: map[string]string{"c": "d"},
		},
	}

	changed := isDiff(src, dst)
	assert.False(t, changed)
}

func TestIsDiffUnstructured(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   "eclipse-che",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "test",
		},
	}

	data, err := yaml.Marshal(pvc)
	assert.NoError(t, err)

	unstructuredPvc := &unstructured.Unstructured{}
	err = yaml.Unmarshal(data, unstructuredPvc)
	assert.NoError(t, err)

	changed := isDiff(pvc, unstructuredPvc)
	assert.False(t, changed)
}
