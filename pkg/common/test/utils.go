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

package test

import (
	"context"
	"os"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TestExpectedResources struct {
	MemoryLimit   string
	MemoryRequest string
	CpuRequest    string
	CpuLimit      string
}

// EnsureReconcile runs the testReconcileFunc until it returns done=true or 10 iterations
func EnsureReconcile(
	t *testing.T,
	ctx *chetypes.DeployContext,
	testReconcileFunc func(ctx *chetypes.DeployContext) (result reconcile.Result, done bool, err error)) {

	for i := 0; i < 10; i++ {
		_, done, err := testReconcileFunc(ctx)
		assert.NoError(t, err)
		if done {
			return
		}
	}

	assert.Fail(t, "Reconcile did not finish in 10 iterations")
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
	assert.Equal(t, corev1.Capability("ALL"), actualDeployment.Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop[0])
	assert.Equal(t, pointer.Bool(false), actualDeployment.Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation)
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

func IsObjectExists(client client.Client, key types.NamespacedName, blueprint client.Object) bool {
	err := client.Get(context.TODO(), key, blueprint)
	if err != nil {
		return false
	}

	return true
}

func GetResourceQuantity(value string, defaultValue string) resource.Quantity {
	if value != "" {
		return resource.MustParse(value)
	}
	return resource.MustParse(defaultValue)
}

func EnableTestMode() {
	_ = os.Setenv("MOCK_API", "1")
}

func IsTestMode() bool {
	testMode := os.Getenv("MOCK_API")
	return len(testMode) != 0
}
