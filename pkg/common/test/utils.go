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
package test

import (
	"context"
	"os"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	console "github.com/openshift/api/console/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	os.Setenv("MOCK_API", "1")
}

func IsTestMode() bool {
	testMode := os.Getenv("MOCK_API")
	return len(testMode) != 0
}

// Initialize DeployContext for tests
func GetDeployContext(cheCluster *chev2.CheCluster, initObjs []runtime.Object) *chetypes.DeployContext {
	if cheCluster == nil {
		// use a default checluster
		cheCluster = &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Status: chev2.CheClusterStatus{
				CheURL: "che-host",
			},
		}
	}

	scheme := scheme.Scheme
	chev2.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	scheme.AddKnownTypes(oauthv1.SchemeGroupVersion, &oauthv1.OAuthClient{})
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{}, &userv1.Identity{})
	scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.OAuth{}, &configv1.Proxy{}, &configv1.Console{})
	scheme.AddKnownTypes(routev1.GroupVersion, &routev1.Route{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	scheme.AddKnownTypes(console.SchemeGroupVersion, &console.ConsoleLink{})

	initObjs = append(initObjs, cheCluster)
	cli := fake.NewFakeClientWithScheme(scheme, initObjs...)
	clientSet := fakeclientset.NewSimpleClientset()
	fakeDiscovery, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)

	return &chetypes.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme,
			DiscoveryClient:  fakeDiscovery,
		},
		Proxy:   &chetypes.Proxy{},
		CheHost: "che-host",
	}
}
