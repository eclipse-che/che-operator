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

package test_client

import (
	projectv1 "github.com/openshift/api/project/v1"
	batchv1 "k8s.io/api/batch/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	securityv1 "github.com/openshift/api/security/v1"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	console "github.com/openshift/api/console/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	templatev1 "github.com/openshift/api/template/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func GetTestClients(initObjs ...client.Object) (client.Client, *fakeDiscovery.FakeDiscovery, *runtime.Scheme) {
	scheme := getScheme()

	fakeClient := fake.
		NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjs...).
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

	return fakeClient, discoveryClient, scheme
}

func getScheme() *runtime.Scheme {
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
