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
	"strings"
	"testing"

	projectv1 "github.com/openshift/api/project/v1"
	batchv1 "k8s.io/api/batch/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1 "github.com/openshift/api/security/v1"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	console "github.com/openshift/api/console/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	templatev1 "github.com/openshift/api/template/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type DeployContextBuild struct {
	cheCluster *chev2.CheCluster
	initObject []client.Object
}

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

func NewCtxBuilder() *DeployContextBuild {
	return &DeployContextBuild{
		initObject: []client.Object{},
		cheCluster: getDefaultCheCluster(),
	}
}

func (f *DeployContextBuild) WithObjects(initObjs ...client.Object) *DeployContextBuild {
	f.initObject = append(f.initObject, initObjs...)
	return f
}

func (f *DeployContextBuild) WithCheCluster(cheCluster *chev2.CheCluster) *DeployContextBuild {
	f.cheCluster = cheCluster
	if f.cheCluster != nil && f.cheCluster.TypeMeta.Kind == "" {
		f.cheCluster.TypeMeta = metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		}
	}
	return f
}

func (f *DeployContextBuild) Build() *chetypes.DeployContext {
	if f.cheCluster != nil {
		f.initObject = append(f.initObject, f.cheCluster)
	}
	scheme := addKnownTypes()

	fakeClient := fake.
		NewClientBuilder().
		WithScheme(scheme).
		WithObjects(f.initObject...).
		WithStatusSubresource(&chev2.CheCluster{}, &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: appsv1.SchemeGroupVersion.String(),
			},
		}).
		Build()

	clientSet := fakeclientset.NewClientset()
	discoveryClient, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
	discoveryClient.Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: "consolelinks"},
			},
		},
		{
			GroupVersion: "che.eclipse.org/v1alpha1",
			APIResources: []metav1.APIResource{
				{
					Name: "kubernetesimagepullers",
				},
			},
		},
	}

	ctx := &chetypes.DeployContext{
		CheCluster: f.cheCluster,
		ClusterAPI: chetypes.ClusterAPI{
			Client:           fakeClient,
			NonCachingClient: fakeClient,
			Scheme:           scheme,
			DiscoveryClient:  discoveryClient,
		},
		Proxy: &chetypes.Proxy{},
	}

	if f.cheCluster != nil {
		ctx.CheHost = strings.TrimPrefix(f.cheCluster.Status.CheURL, "https://")
	}

	return ctx
}

func getDefaultCheCluster() *chev2.CheCluster {
	return &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}
}

func addKnownTypes() *runtime.Scheme {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(controllerv1alpha1.GroupVersion, &controllerv1alpha1.DevWorkspaceOperatorConfig{}, &controllerv1alpha1.DevWorkspaceOperatorConfigList{})
	scheme.AddKnownTypes(controllerv1alpha1.GroupVersion, &controllerv1alpha1.DevWorkspaceRouting{}, &controllerv1alpha1.DevWorkspaceRoutingList{})
	scheme.AddKnownTypes(oauthv1.GroupVersion, &oauthv1.OAuthClient{}, &oauthv1.OAuthClientList{})
	scheme.AddKnownTypes(configv1.GroupVersion, &configv1.Proxy{}, &configv1.Console{})
	scheme.AddKnownTypes(templatev1.GroupVersion, &templatev1.Template{}, &templatev1.TemplateList{})
	scheme.AddKnownTypes(routev1.GroupVersion, &routev1.Route{}, &routev1.RouteList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{}, &corev1.SecretList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ConfigMap{}, &corev1.ConfigMapList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Service{}, &corev1.ServiceList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.ServiceAccount{}, &corev1.ServiceAccountList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Pod{}, &corev1.PodList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Namespace{}, &corev1.NamespaceList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.PersistentVolumeClaim{}, &corev1.PersistentVolumeClaimList{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.LimitRange{}, &corev1.LimitRangeList{})
	scheme.AddKnownTypes(console.GroupVersion, &console.ConsoleLink{})
	scheme.AddKnownTypes(chev1alpha1.GroupVersion, &chev1alpha1.KubernetesImagePuller{})
	scheme.AddKnownTypes(securityv1.GroupVersion, &securityv1.SecurityContextConstraints{})
	scheme.AddKnownTypes(rbacv1.SchemeGroupVersion, &rbacv1.Role{}, &rbacv1.RoleList{})
	scheme.AddKnownTypes(rbacv1.SchemeGroupVersion, &rbacv1.RoleBinding{}, &rbacv1.RoleBindingList{})
	scheme.AddKnownTypes(rbacv1.SchemeGroupVersion, &rbacv1.ClusterRole{}, &rbacv1.ClusterRoleList{})
	scheme.AddKnownTypes(rbacv1.SchemeGroupVersion, &rbacv1.ClusterRoleBinding{}, &rbacv1.ClusterRoleBindingList{})
	scheme.AddKnownTypes(appsv1.SchemeGroupVersion, &appsv1.Deployment{}, &appsv1.DeploymentList{})
	scheme.AddKnownTypes(chev2.GroupVersion, &chev2.CheCluster{}, &chev2.CheClusterList{})
	scheme.AddKnownTypes(networkingv1.SchemeGroupVersion, &networkingv1.Ingress{}, &networkingv1.IngressList{})
	scheme.AddKnownTypes(batchv1.SchemeGroupVersion, &batchv1.Job{}, &batchv1.JobList{})
	scheme.AddKnownTypes(projectv1.SchemeGroupVersion, &projectv1.Project{}, &projectv1.ProjectList{})

	return scheme
}
