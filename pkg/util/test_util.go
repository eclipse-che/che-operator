//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package util

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type TestExpectedResources struct {
	MemoryLimit   string
	MemoryRequest string
	CpuRequest    string
	CpuLimit      string
}

func CompareResources(actualDeployment *appsv1.Deployment, expected TestExpectedResources, t *testing.T) {
	container := &actualDeployment.Spec.Template.Spec.Containers[0]
	compareQuantity(
		"Memory limits",
		container.Resources.Limits.Memory(),
		expected.MemoryLimit,
		t,
	)

	compareQuantity(
		"Memory requests",
		container.Resources.Requests.Memory(),
		expected.MemoryRequest,
		t,
	)

	compareQuantity(
		"CPU limits",
		container.Resources.Limits.Cpu(),
		expected.CpuLimit,
		t,
	)

	compareQuantity(
		"CPU requests",
		container.Resources.Requests.Cpu(),
		expected.CpuRequest,
		t,
	)
}

func ValidateSecurityContext(actualDeployment *appsv1.Deployment, t *testing.T) {
	if actualDeployment.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop[0] != "ALL" {
		t.Error("Deployment doesn't contain 'Capabilities Drop ALL' in a SecurityContext")
	}
}

func compareQuantity(resource string, actualQuantity *resource.Quantity, expected string, t *testing.T) {
	expectedQuantity := GetResourceQuantity(expected, expected)
	if !actualQuantity.Equal(expectedQuantity) {
		t.Errorf("%s: expected %s, actual %s", resource, expectedQuantity.String(), actualQuantity.String())
	}
}

func ValidateContainData(actualData map[string]string, expectedData map[string]string, t *testing.T) {
	for k, v := range expectedData {
		actualValue, exists := actualData[k]
		if exists {
			if actualValue != v {
				t.Errorf("Key '%s', actual: '%s', expected: '%s'", k, actualValue, v)
			}
		} else if v != "" {
			t.Errorf("Key '%s' does not exists, expected value: '%s'", k, v)
		}
	}
}

func FindVolume(volumes []corev1.Volume, name string) corev1.Volume {
	for _, volume := range volumes {
		if volume.Name == name {
			return volume
		}
	}

	return corev1.Volume{}
}

func FindVolumeMount(volumes []corev1.VolumeMount, name string) corev1.VolumeMount {
	for _, volumeMount := range volumes {
		if volumeMount.Name == name {
			return volumeMount
		}
	}

	return corev1.VolumeMount{}
}
