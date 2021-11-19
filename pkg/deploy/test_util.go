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
package deploy

import (
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	configv1 "github.com/openshift/api/config/v1"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Initialize DeployContext for tests
func GetTestDeployContext(cheCluster *orgv1.CheCluster, initObjs []runtime.Object) *DeployContext {
	if cheCluster == nil {
		// use a default checluster
		cheCluster = &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "org.eclipse.che/v1",
				Kind:       "CheCluster",
			},
		}
	}

	scheme := scheme.Scheme
	orgv1.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})
	scheme.AddKnownTypes(operatorsv1alpha1.SchemeGroupVersion, &operatorsv1alpha1.Subscription{})
	scheme.AddKnownTypes(oauthv1.SchemeGroupVersion, &oauthv1.OAuthClient{})
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{}, &userv1.Identity{})
	scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.OAuth{}, &configv1.Proxy{})
	scheme.AddKnownTypes(routev1.GroupVersion, &routev1.Route{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})

	initObjs = append(initObjs, cheCluster)
	cli := fake.NewFakeClientWithScheme(scheme, initObjs...)
	clientSet := fakeclientset.NewSimpleClientset()
	fakeDiscovery, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)

	return &DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme,
			DiscoveryClient:  fakeDiscovery,
		},
		Proxy: &Proxy{},
	}
}
