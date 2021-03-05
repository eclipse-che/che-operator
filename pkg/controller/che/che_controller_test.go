//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package che

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	mocks "github.com/eclipse/che-operator/mocks"

	"reflect"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
	"github.com/golang/mock/gomock"

	identity_provider "github.com/eclipse/che-operator/pkg/deploy/identity-provider"
	"github.com/google/go-cmp/cmp"

	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"

	console "github.com/openshift/api/console/v1"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	oauth_config "github.com/openshift/api/config/v1"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"

	che_mocks "github.com/eclipse/che-operator/mocks/pkg/controller/che"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/yaml"

	"testing"
)

var (
	name            = "eclipse-che"
	namespace       = "eclipse-che"
	csvName         = "kubernetes-imagepuller-operator.v0.0.4"
	packageManifest = &packagesv1.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Status: packagesv1.PackageManifestStatus{
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "olm",
			DefaultChannel:         "stable",
			PackageName:            "kubernetes-imagepuller-operator",
		},
	}
	operatorGroup = &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{
				namespace,
			},
		},
	}
	subscription = &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          "community-operators",
			Channel:                "stable",
			CatalogSourceNamespace: "olm",
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
			Package:                "kubernetes-imagepuller-operator",
		},
	}
	wrongSubscription = &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          "community-operators",
			Channel:                "beta",
			CatalogSourceNamespace: "olm",
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
			Package:                "kubernetes-imagepuller-operator",
		},
	}
	valueTrue          = true
	defaultImagePuller = &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "che.eclipse.org/v1alpha1",
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che-image-puller",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": name,
				"app":                       "che",
				"component":                 "kubernetes-image-puller",
			},
			ResourceVersion: "1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "org.eclipse.che/v1",
					Kind:               "CheCluster",
					Controller:         &valueTrue,
					BlockOwnerDeletion: &valueTrue,
					Name:               "eclipse-che",
				},
			},
		},
		Spec: chev1alpha1.KubernetesImagePullerSpec{
			DeploymentName: "kubernetes-image-puller",
			ConfigMapName:  "k8s-image-puller",
		},
	}
	clusterServiceVersion = &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      csvName,
		},
	}
	nonEmptyUserList = &userv1.UserList{
		Items: []userv1.User{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user2",
				},
			},
		},
	}
	oAuthClient                  = &oauth.OAuthClient{}
	oAuthWithNoIdentityProviders = &oauth_config.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: namespace,
		},
	}
	oAuthWithIdentityProvider = &oauth_config.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: namespace,
		},
		Spec: oauth_config.OAuthSpec{
			IdentityProviders: []oauth_config.IdentityProvider{
				{
					Name: "htpasswd",
				},
			},
		},
	}
	route = &routev1.Route{}
)

func init() {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile("../../../deploy/operator.yaml")
	yaml.Unmarshal(data, operator)
	if err == nil {
		for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
			os.Setenv(env.Name, env.Value)
		}
	}
}

func TestCaseAutoDetectOAuth(t *testing.T) {
	type testCase struct {
		name                                string
		initObjects                         []runtime.Object
		openshiftVersion                    string
		initialOAuthValue                   *bool
		oAuthExpected                       *bool
		initialOpenShiftOAuthUserEnabled    *bool
		OpenShiftOAuthUserCredentialsSecret string
		mockFunction                        func(ctrl *gomock.Controller, crNamespace string, usernamePrefix string) *che_mocks.MockOpenShiftOAuthUserHandler
	}

	testCases := []testCase{
		{
			name: "che-operator should auto enable oAuth when Che CR with oAuth nil value on the Openshift 3 with users > 0",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauth.OAuthClient{},
			},
			openshiftVersion:  "3",
			initialOAuthValue: nil,
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should auto disable oAuth when Che CR with nil oAuth on the Openshift 3 with no users",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauth.OAuthClient{},
			},
			openshiftVersion:  "3",
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = true even if there no users on the Openshift 3",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauth.OAuthClient{},
			},
			openshiftVersion:  "3",
			initialOAuthValue: util.NewBoolPointer(true),
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there are some users on the Openshift 3",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauth.OAuthClient{},
			},
			openshiftVersion:  "3",
			initialOAuthValue: util.NewBoolPointer(true),
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = false even if there are some users on the Openshift 3",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauth.OAuthClient{},
			},
			openshiftVersion:  "3",
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = false even if no users on the Openshift 3",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauth.OAuthClient{},
			},
			openshiftVersion:  "3",
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should auto enable oAuth when Che CR with nil value on the Openshift 4 with identity providers",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
			},
			openshiftVersion:  "4",
			initialOAuthValue: nil,
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
			},
			openshiftVersion:                 "4",
			initialOAuthValue:                util.NewBoolPointer(true),
			oAuthExpected:                    util.NewBoolPointer(true),
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(true),
		},
		{
			name: "che-operator should create initial user and enable oAuth, when oAuth = true, initialOpenShiftOAuthUserEnabled = true and there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
			},
			openshiftVersion:                 "4",
			initialOAuthValue:                nil,
			oAuthExpected:                    util.NewBoolPointer(true),
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(true),
			mockFunction: func(ctrl *gomock.Controller, crNamespace string, userNamePrefix string) *che_mocks.MockOpenShiftOAuthUserHandler {
				m := che_mocks.NewMockOpenShiftOAuthUserHandler(ctrl)
				m.EXPECT().SyncOAuthInitialUser(gomock.Any(), gomock.Any()).Return(true, nil)
				return m
			},
			OpenShiftOAuthUserCredentialsSecret: openShiftOAuthUserCredentialsSecret,
		},
		{
			name: "che-operator should respect oAuth = true even if there are some users on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
			},
			openshiftVersion:                 "4",
			initialOAuthValue:                util.NewBoolPointer(true),
			oAuthExpected:                    util.NewBoolPointer(true),
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = false even if there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
			},
			openshiftVersion:  "4",
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = false even if there are some users on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
			},
			openshiftVersion:  "4",
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name:                             "che-operator should auto disable oAuth on error retieve identity providers",
			initObjects:                      []runtime.Object{},
			openshiftVersion:                 "4",
			initialOAuthValue:                nil,
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(true),
			oAuthExpected:                    util.NewBoolPointer(false),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
			scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
			scheme.AddKnownTypes(oauth_config.SchemeGroupVersion, &oauth_config.OAuth{})
			scheme.AddKnownTypes(routev1.GroupVersion, route)
			initCR := InitCheWithSimpleCR().DeepCopy()
			initCR.Spec.Auth.OpenShiftoAuth = testCase.initialOAuthValue
			testCase.initObjects = append(testCase.initObjects, initCR)
			initCR.Spec.Auth.InitialOpenShiftOAuthUser = testCase.initialOpenShiftOAuthUserEnabled

			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			if !ok {
				t.Fatal("Error creating fake discovery client")
			}

			// prepare mocks
			var userHandlerMock *che_mocks.MockOpenShiftOAuthUserHandler
			if testCase.mockFunction != nil {
				ctrl := gomock.NewController(t)
				userHandlerMock = testCase.mockFunction(ctrl, initCR.Namespace, deploy.DefaultCheFlavor(initCR))
				defer ctrl.Finish()
			}

			r := &ReconcileChe{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				scheme:          scheme,
				tests:           true,
				userHandler:     userHandlerMock,
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}

			os.Setenv("OPENSHIFT_VERSION", testCase.openshiftVersion)

			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}

			cheCR := &orgv1.CheCluster{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCR); err != nil {
				t.Errorf("CR not found")
			}

			if cheCR.Spec.Auth.OpenShiftoAuth == nil {
				t.Error("OAuth should not stay with nil value.")
			}

			if *cheCR.Spec.Auth.OpenShiftoAuth != *testCase.oAuthExpected {
				t.Errorf("Openshift oAuth should be %t", *testCase.oAuthExpected)
			}

			if cheCR.Status.OpenShiftOAuthUserCredentialsSecret != testCase.OpenShiftOAuthUserCredentialsSecret {
				t.Errorf("Expected initial openshift oAuth user secret %s in the CR status", testCase.OpenShiftOAuthUserCredentialsSecret)
			}
		})
	}
}

func TestImagePullerConfiguration(t *testing.T) {
	type testCase struct {
		name                  string
		initCR                *orgv1.CheCluster
		initObjects           []runtime.Object
		expectedCR            *orgv1.CheCluster
		expectedOperatorGroup *operatorsv1.OperatorGroup
		expectedSubscription  *operatorsv1alpha1.Subscription
		expectedImagePuller   *chev1alpha1.KubernetesImagePuller
		shouldDelete          bool
	}

	testCases := []testCase{
		{
			name:   "image puller enabled, no operatorgroup, should create an operatorgroup",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				packageManifest,
			},
			expectedOperatorGroup: operatorGroup,
		},
		{
			name:   "image puller enabled, operatorgroup exists, should create a subscription",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
			},
			expectedSubscription: subscription,
		},
		{
			name:   "image puller enabled, subscription created but has changed, should update subscription, this shouldn't happen",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				wrongSubscription,
			},
			expectedSubscription: subscription,
		},
		{
			name:       "image puller enabled, subscription created, should add finalizer",
			initCR:     InitCheCRWithImagePullerEnabled(),
			expectedCR: ExpectedCheCRWithImagePullerFinalizer(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
			},
		},
		{
			name:   "image puller enabled with finalizer but default values are empty, subscription exists, should update the CR",
			initCR: InitCheCRWithImagePullerFinalizer(),
			expectedCR: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            name,
					Namespace:       namespace,
					ResourceVersion: "1",
					Finalizers: []string{
						"kubernetesimagepullers.finalizers.che.eclipse.org",
					},
				},
				Spec: orgv1.CheClusterSpec{
					ImagePuller: orgv1.CheClusterSpecImagePuller{
						Enable: true,
						Spec: chev1alpha1.KubernetesImagePullerSpec{
							DeploymentName: "kubernetes-image-puller",
							ConfigMapName:  "k8s-image-puller",
						},
					},
				},
			},
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
			},
		},
		{
			name:   "image puller enabled default values already set, subscription exists, should create a KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndDefaultValuesSet(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
			},
			expectedImagePuller: defaultImagePuller,
		},
		{
			name:   "image puller enabled, KubernetesImagePuller created and spec in CheCluster is different, should update the KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndNewValuesSet(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				defaultImagePuller,
			},
			expectedImagePuller: &chev1alpha1.KubernetesImagePuller{
				TypeMeta: metav1.TypeMeta{Kind: "KubernetesImagePuller", APIVersion: "che.eclipse.org/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "2",
					Name:            name + "-image-puller",
					Namespace:       namespace,
					Labels: map[string]string{
						"app":                       "che",
						"component":                 "kubernetes-image-puller",
						"app.kubernetes.io/part-of": "eclipse-che",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "org.eclipse.che/v1",
							Kind:               "CheCluster",
							BlockOwnerDeletion: &valueTrue,
							Controller:         &valueTrue,
							Name:               name,
						},
					},
				},
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					ConfigMapName:  "k8s-image-puller-trigger-update",
					DeploymentName: "kubernetes-image-puller-trigger-update",
				},
			},
		},
		{
			name:   "image puller already created, imagePuller disabled, should delete everything",
			initCR: InitCheCRWithImagePullerDisabled(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				clusterServiceVersion,
				defaultImagePuller,
			},
			shouldDelete: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			packagesv1.AddToScheme(scheme.Scheme)
			operatorsv1alpha1.AddToScheme(scheme.Scheme)
			operatorsv1.AddToScheme(scheme.Scheme)
			chev1alpha1.AddToScheme(scheme.Scheme)
			routev1.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initCR)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "packages.operators.coreos.com/v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "PackageManifest",
						},
					},
				},
				{
					GroupVersion: "operators.coreos.com/v1alpha1",
					APIResources: []metav1.APIResource{
						{Kind: "OperatorGroup"},
						{Kind: "Subscription"},
						{Kind: "ClusterServiceVersion"},
					},
				},
				{
					GroupVersion: "che.eclipse.org/v1alpha1",
					APIResources: []metav1.APIResource{
						{Kind: "KubernetesImagePuller"},
					},
				},
			}

			if !ok {
				t.Error("Error creating fake discovery client")
				os.Exit(1)
			}

			r := &ReconcileChe{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				scheme:          scheme.Scheme,
				tests:           true,
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}
			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}

			if testCase.expectedOperatorGroup != nil {
				gotOperatorGroup := &operatorsv1.OperatorGroup{}
				err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedOperatorGroup.Namespace, Name: testCase.expectedOperatorGroup.Name}, gotOperatorGroup)
				if err != nil {
					t.Errorf("Error getting OperatorGroup: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedOperatorGroup.Spec.TargetNamespaces, gotOperatorGroup.Spec.TargetNamespaces) {
					t.Errorf("Error expected target namespace %v but got %v", testCase.expectedOperatorGroup.Spec.TargetNamespaces, gotOperatorGroup.Spec.TargetNamespaces)
				}
			}
			if testCase.expectedSubscription != nil {
				gotSubscription := &operatorsv1alpha1.Subscription{}
				err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedSubscription.Namespace, Name: testCase.expectedSubscription.Name}, gotSubscription)
				if err != nil {
					t.Errorf("Error getting Subscription: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedSubscription.Spec, gotSubscription.Spec) {
					t.Errorf("Error, subscriptions differ (-want +got) %v", cmp.Diff(testCase.expectedSubscription.Spec, gotSubscription.Spec))
				}
			}
			// if expectedCR is not set, don't check it
			if testCase.expectedCR != nil && !reflect.DeepEqual(testCase.initCR, testCase.expectedCR) {
				gotCR := &orgv1.CheCluster{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, gotCR)
				if err != nil {
					t.Errorf("Error getting CheCluster: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedCR, gotCR) {
					t.Errorf("Expected CR and CR returned from API server are different (-want +got): %v", cmp.Diff(testCase.expectedCR, gotCR))
				}
			}
			if testCase.expectedImagePuller != nil {
				gotImagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedImagePuller.Namespace, Name: testCase.expectedImagePuller.Name}, gotImagePuller)
				if err != nil {
					t.Errorf("Error getting KubernetesImagePuller: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedImagePuller, gotImagePuller) {
					t.Errorf("Expected KubernetesImagePuller and KubernetesImagePuller returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedImagePuller, gotImagePuller))
				}
			}
			if testCase.shouldDelete {

				imagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name + "-image-puller"}, imagePuller)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found KubernetesImagePuller: %v", err)
				}

				clusterServiceVersion := &operatorsv1alpha1.ClusterServiceVersion{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: csvName}, clusterServiceVersion)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found ClusterServiceVersion: %v", err)
				}

				subscription := &operatorsv1alpha1.Subscription{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, subscription)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found subscription: %v", err)
				}

				operatorGroup := &operatorsv1.OperatorGroup{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, operatorGroup)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found subscription: %v", err)
				}
			}
		})
	}
}

func TestCheController(t *testing.T) {
	os.Setenv("OPENSHIFT_VERSION", "3")
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &ReconcileChe{client: cl, nonCachedClient: cl, scheme: &scheme, discoveryClient: dc, tests: true}

	// get CR
	cheCR := &orgv1.CheCluster{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get devfile-registry configmap
	devfilecm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DevfileRegistryName, Namespace: cheCR.Namespace}, devfilecm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", devfilecm.Name, err)
	}

	// update CR and make sure Che configmap has been updated
	cheCR.Spec.Server.TlsSupport = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	// reconcile again
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get configmap
	cm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cm.Name, err)
	}

	customCm := &corev1.ConfigMap{}

	// Custom ConfigMap should be gone
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "custom", Namespace: cheCR.Namespace}, customCm)
	if !errors.IsNotFound(err) {
		t.Errorf("Custom config map should be deleted and merged with Che ConfigMap")
	}

	// Get the custom role binding that should have been created for the role we passed in
	rb := &rbac.RoleBinding{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che-workspace-custom", Namespace: cheCR.Namespace}, rb); err != nil {
		t.Errorf("Custom role binding %s not found: %s", rb.Name, err)
	}

	// run a few checks to make sure the operator reconciled tls routes and updated configmap
	if cm.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"] != "true" {
		t.Errorf("ConfigMap wasn't updated. Extecting true, got: %s", cm.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"])
	}
	route := &routev1.Route{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultCheFlavor(cheCR), Namespace: cheCR.Namespace}, route); err != nil {
		t.Errorf("Route %s not found: %s", cm.Name, err)
	}
	if route.Spec.TLS.Termination != "edge" {
		t.Errorf("Test failed as %s %s is not a TLS route", route.Kind, route.Name)
	}

	// update CR and make sure Che configmap has been updated
	cheCR.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(true)
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get configmap and check if identity provider name and workspace project name are correctly set
	cm = &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cm.Name, err)
	}
	if cm.Data["CHE_INFRA_OPENSHIFT_PROJECT"] != "" {
		t.Errorf("ConfigMap wasn't updated properly. Extecting empty string, got: '%s'", cm.Data["CHE_INFRA_OPENSHIFT_PROJECT"])
	}

	_, isOpenshiftv4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("Error detecting openshift version: %v", err)
	}
	expectedIdentityProviderName := "openshift-v3"
	if isOpenshiftv4 {
		expectedIdentityProviderName = "openshift-v4"
	}

	if cm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"] != expectedIdentityProviderName {
		t.Errorf("ConfigMap wasn't updated properly. Expecting '%s', got: '%s'", expectedIdentityProviderName, cm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"])
	}

	clusterAPI := deploy.ClusterAPI{
		Client:          r.client,
		NonCachedClient: r.client,
		Scheme:          r.scheme,
	}

	deployContext := &deploy.DeployContext{
		CheCluster: cheCR,
		ClusterAPI: clusterAPI,
	}

	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: cheCR.Name, Namespace: cheCR.Namespace}, cheCR); err != nil {
		t.Errorf("Failed to get the Che custom resource %s: %s", cheCR.Name, err)
	}
	if _, err = identity_provider.SyncOpenShiftIdentityProviderItems(deployContext); err != nil {
		t.Errorf("Failed to create the items for the identity provider: %s", err)
	}
	oAuthClientName := cheCR.Spec.Auth.OAuthClientName
	oauthSecret := cheCR.Spec.Auth.OAuthSecret
	oAuthClient := &oauth.OAuthClient{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName, Namespace: ""}, oAuthClient); err != nil {
		t.Errorf("Failed to Get oAuthClient %s: %s", oAuthClient.Name, err)
	}
	if oAuthClient.Secret != oauthSecret {
		t.Errorf("Secrets do not match. Expecting %s, got %s", oauthSecret, oAuthClient.Secret)
	}

	// check if a new Postgres deployment is not created when spec.Database.ExternalDB is true
	cheCR.Spec.Database.ExternalDb = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}
	postgresDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.PostgresName, Namespace: cheCR.Namespace}, postgresDeployment)
	err = r.client.Delete(context.TODO(), postgresDeployment)
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.PostgresName, Namespace: cheCR.Namespace}, postgresDeployment)
	if err == nil {
		t.Fatalf("Deployment postgres shoud not exist")
	}

	// check of storageClassName ends up in pvc spec
	fakeStorageClassName := "fake-storage-class-name"
	cheCR.Spec.Storage.PostgresPVCStorageClassName = fakeStorageClassName
	cheCR.Spec.Database.ExternalDb = false
	if err := r.client.Update(context.TODO(), cheCR); err != nil {
		t.Fatalf("Failed to update %s CR: %s", cheCR.Name, err)
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: cheCR.Namespace}, pvc); err != nil {
		t.Fatalf("Failed to get PVC: %s", err)
	}
	if err = r.client.Delete(context.TODO(), pvc); err != nil {
		t.Fatalf("Failed to delete PVC %s: %s", pvc.Name, err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	pvc = &corev1.PersistentVolumeClaim{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: cheCR.Namespace}, pvc); err != nil {
		t.Fatalf("Failed to get PVC: %s", err)
	}
	actualStorageClassName := pvc.Spec.StorageClassName
	if len(*actualStorageClassName) != len(fakeStorageClassName) {
		t.Fatalf("Expecting %s storageClassName, got %s", fakeStorageClassName, *actualStorageClassName)
	}

	// check if oAuthClient is deleted after CR is deleted (finalizer logic)
	// since fake api does not set deletion timestamp, CR is updated in tests rather than deleted
	logrus.Info("Updating CR with deletion timestamp")
	deletionTimestamp := &metav1.Time{Time: time.Now()}
	cheCR.DeletionTimestamp = deletionTimestamp
	if err := r.client.Update(context.TODO(), cheCR); err != nil {
		t.Fatalf("Failed to update CR: %s", err)
	}
	if err := r.ReconcileFinalizer(cheCR); err != nil {
		t.Fatal("Failed to reconcile oAuthClient")
	}
	oauthClientName := cheCR.Spec.Auth.OAuthClientName
	_, err = r.GetOAuthClient(oauthClientName)
	if err == nil {
		t.Fatalf("OauthClient %s has not been deleted", oauthClientName)
	}
	logrus.Infof("Disregard the error above. OauthClient %s has been deleted", oauthClientName)
}

func TestConfiguringLabelsForRoutes(t *testing.T) {
	os.Setenv("OPENSHIFT_VERSION", "3")
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &ReconcileChe{client: cl, nonCachedClient: cl, scheme: &scheme, discoveryClient: dc, tests: true}

	// get CR
	cheCR := &orgv1.CheCluster{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	// reconcile
	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	cheCR.Spec.Server.CheServerRoute.Labels = "route=one"
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	// reconcile again
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get route
	route := &routev1.Route{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultCheFlavor(cheCR), Namespace: cheCR.Namespace}, route); err != nil {
		t.Errorf("Route %s not found: %s", route.Name, err)
	}

	if route.ObjectMeta.Labels["route"] != "one" {
		t.Fatalf("Route '%s' does not have label '%s'", route.Name, route)
	}
}

func TestShouldDelegatePermissionsForCheWorkspaces(t *testing.T) {
	os.Setenv("OPENSHIFT_VERSION", "3")
	type testCase struct {
		name        string
		initObjects []runtime.Object

		clusterRole bool
		checluster  *orgv1.CheCluster
	}

	// the same namespace with Che
	crWsInTheSameNs1 := InitCheWithSimpleCR().DeepCopy()
	crWsInTheSameNs1.Spec.Server.WorkspaceNamespaceDefault = crWsInTheSameNs1.Namespace

	crWsInTheSameNs2 := InitCheWithSimpleCR().DeepCopy()
	crWsInTheSameNs2.Spec.Server.WorkspaceNamespaceDefault = ""

	crWsInTheSameNs3 := InitCheWithSimpleCR().DeepCopy()
	crWsInTheSameNs3.Spec.Server.CustomCheProperties = make(map[string]string)
	crWsInTheSameNs3.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"] = ""

	crWsInTheSameNs4 := InitCheWithSimpleCR().DeepCopy()
	crWsInTheSameNs4.Spec.Server.CustomCheProperties = make(map[string]string)
	crWsInTheSameNs4.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"] = crWsInTheSameNs1.Namespace

	// differ namespace with Che
	crWsInAnotherNs1 := InitCheWithSimpleCR().DeepCopy()
	crWsInAnotherNs1.Spec.Server.WorkspaceNamespaceDefault = "some-test-namespace"

	crWsInAnotherNs2 := InitCheWithSimpleCR().DeepCopy()
	crWsInAnotherNs2.Spec.Server.CustomCheProperties = make(map[string]string)
	crWsInAnotherNs2.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"] = "some-test-namespace"

	crWsInAnotherNs3 := InitCheWithSimpleCR().DeepCopy()
	crWsInAnotherNs3.Spec.Server.CustomCheProperties = make(map[string]string)
	crWsInAnotherNs3.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"] = crWsInTheSameNs1.Namespace
	crWsInAnotherNs3.Spec.Server.WorkspaceNamespaceDefault = "some-test-namespace"

	testCases := []testCase{
		{
			name:        "che-operator should delegate permission for workspaces in the same namespace with Che. WorkspaceNamespaceDefault=" + crWsInTheSameNs1.Namespace,
			initObjects: []runtime.Object{},
			clusterRole: false,
			checluster:  crWsInTheSameNs1,
		},
		{
			name:        "che-operator should delegate permission for workspaces in the same namespace with Che. WorkspaceNamespaceDefault=''",
			initObjects: []runtime.Object{},
			clusterRole: false,
			checluster:  crWsInTheSameNs2,
		},
		{
			name:        "che-operator should delegate permission for workspaces in the same namespace with Che. Property CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT=''",
			initObjects: []runtime.Object{},
			clusterRole: false,
			checluster:  crWsInTheSameNs3,
		},
		{
			name:        "che-operator should delegate permission for workspaces in the same namespace with Che. Property CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT=" + crWsInTheSameNs1.Namespace,
			initObjects: []runtime.Object{},
			clusterRole: false,
			checluster:  crWsInTheSameNs4,
		},
		{
			name:        "che-operator should delegate permission for workspaces in differ namespace than Che. WorkspaceNamespaceDefault = 'some-test-namespace'",
			initObjects: []runtime.Object{},
			clusterRole: true,
			checluster:  crWsInAnotherNs1,
		},
		{
			name:        "che-operator should delegate permission for workspaces in differ namespace than Che. Property CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT = 'some-test-namespace'",
			initObjects: []runtime.Object{},
			clusterRole: true,
			checluster:  crWsInAnotherNs2,
		},
		{
			name:        "che-operator should delegate permission for workspaces in differ namespace than Che. Property CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT points to Che namespace with higher priority WorkspaceNamespaceDefault = 'some-test-namespace'.",
			initObjects: []runtime.Object{},
			clusterRole: false,
			checluster:  crWsInAnotherNs3,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
			scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
			scheme.AddKnownTypes(oauth_config.SchemeGroupVersion, &oauth_config.OAuth{})
			scheme.AddKnownTypes(routev1.GroupVersion, route)

			initCR := testCase.checluster
			initCR.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(false)
			testCase.initObjects = append(testCase.initObjects, initCR)

			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			// todo do we need fake discovery
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			if !ok {
				t.Fatal("Error creating fake discovery client")
			}

			var m *mocks.MockPermissionChecker
			if testCase.clusterRole {
				ctrl := gomock.NewController(t)
				m = mocks.NewMockPermissionChecker(ctrl)
				m.EXPECT().GetNotPermittedPolicyRules(gomock.Any(), "").Return([]rbac.PolicyRule{}, nil).MaxTimes(2)
				defer ctrl.Finish()
			}

			r := &ReconcileChe{
				client:            cli,
				nonCachedClient:   nonCachedClient,
				discoveryClient:   fakeDiscovery,
				scheme:            scheme,
				permissionChecker: m,
				tests:             true,
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}

			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}
			_, err = r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}

			if !testCase.clusterRole {
				viewRole := &rbac.Role{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.ViewRoleName, Namespace: namespace}, viewRole); err != nil {
					t.Errorf("role '%s' not found", deploy.ViewRoleName)
				}
				viewRoleBinding := &rbac.RoleBinding{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: ViewRoleBindingName, Namespace: namespace}, viewRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", ViewRoleBindingName)
				}

				execRole := &rbac.Role{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.ExecRoleName, Namespace: namespace}, execRole); err != nil {
					t.Errorf("role '%s' not found", deploy.ExecRoleName)
				}
				execRoleBinding := &rbac.RoleBinding{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: ExecRoleBindingName, Namespace: namespace}, execRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", ExecRoleBindingName)
				}

				editRoleBinding := &rbac.RoleBinding{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: EditRoleBindingName, Namespace: namespace}, editRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", EditRoleBindingName)
				}
			} else {
				manageNamespacesClusterRoleName := fmt.Sprintf(CheWorkspacesNamespaceClusterRoleNameTemplate, namespace)
				cheManageNamespaceClusterRole := &rbac.ClusterRole{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: manageNamespacesClusterRoleName}, cheManageNamespaceClusterRole); err != nil {
					t.Errorf("role '%s' not found", manageNamespacesClusterRoleName)
				}
				cheManageNamespaceClusterRoleBinding := &rbac.ClusterRoleBinding{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: manageNamespacesClusterRoleName}, cheManageNamespaceClusterRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", manageNamespacesClusterRoleName)
				}

				cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, namespace)
				cheWorkspacesClusterRole := &rbac.ClusterRole{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: cheWorkspacesClusterRoleName}, cheWorkspacesClusterRole); err != nil {
					t.Errorf("role '%s' not found", cheWorkspacesClusterRole)
				}
				cheWorkspacesClusterRoleBinding := &rbac.ClusterRoleBinding{}
				if err := r.client.Get(context.TODO(), types.NamespacedName{Name: cheWorkspacesClusterRoleName}, cheWorkspacesClusterRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", cheWorkspacesClusterRole)
				}
			}
		})
	}
}

func TestShouldFallBackWorspaceNamespaceDefaultBecauseNotEnoughtPermissions(t *testing.T) {
	// the same namespace with Che
	cr := InitCheWithSimpleCR().DeepCopy()
	cr.Spec.Server.WorkspaceNamespaceDefault = "che-workspace-<username>"

	logf.SetLogger(zap.LoggerTo(os.Stdout, true))

	scheme := scheme.Scheme
	orgv1.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
	scheme.AddKnownTypes(oauth_config.SchemeGroupVersion, &oauth_config.OAuth{})
	scheme.AddKnownTypes(routev1.GroupVersion, route)

	cr.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(false)

	cli := fake.NewFakeClientWithScheme(scheme, cr)
	nonCachedClient := fake.NewFakeClientWithScheme(scheme, cr)
	clientSet := fakeclientset.NewSimpleClientset()
	// todo do we need fake discovery
	fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
	fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

	if !ok {
		t.Fatal("Error creating fake discovery client")
	}

	var m *mocks.MockPermissionChecker
	ctrl := gomock.NewController(t)
	m = mocks.NewMockPermissionChecker(ctrl)
	m.EXPECT().GetNotPermittedPolicyRules(gomock.Any(), "").Return([]rbac.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create", "update"},
		},
	}, nil).MaxTimes(2)
	defer ctrl.Finish()

	r := &ReconcileChe{
		client:            cli,
		nonCachedClient:   nonCachedClient,
		discoveryClient:   fakeDiscovery,
		scheme:            scheme,
		permissionChecker: m,
		tests:             true,
	}
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("Error reconciling: %v", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("Error reconciling: %v", err)
	}

	cheCluster := &orgv1.CheCluster{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCluster); err != nil {
		t.Errorf("Unable to get checluster")
	}
	if cheCluster.Spec.Server.WorkspaceNamespaceDefault != namespace {
		t.Error("Failed fallback workspaceNamespaceDefault to execute workspaces in the same namespace with Che")
	}
}

func Init() (client.Client, discovery.DiscoveryInterface, runtime.Scheme) {
	objs, ds, scheme := createAPIObjects()

	oAuthClient := &oauth.OAuthClient{}
	users := &userv1.UserList{}
	user := &userv1.User{}

	// Register operator types with the runtime scheme
	scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, users, user)

	// Create a fake client to mock API calls
	return fake.NewFakeClient(objs...), ds, scheme
}

func createAPIObjects() ([]runtime.Object, discovery.DiscoveryInterface, runtime.Scheme) {
	pgPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-pg-pod",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"component": deploy.PostgresName,
			},
		},
	}

	// A CheCluster custom resource with metadata and spec
	cheCR := InitCheWithSimpleCR()

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.DefaultCheFlavor(cheCR),
			Namespace: namespace,
		},
	}

	packageManifest := &packagesv1.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		cheCR, pgPod, route, packageManifest,
	}

	// Register operator types with the runtime scheme
	scheme := scheme.Scheme
	scheme.AddKnownTypes(orgv1.SchemeGroupVersion, cheCR)
	scheme.AddKnownTypes(routev1.SchemeGroupVersion, route)
	scheme.AddKnownTypes(console.GroupVersion, &console.ConsoleLink{})
	chev1alpha1.AddToScheme(scheme)
	packagesv1.AddToScheme(scheme)
	operatorsv1.AddToScheme(scheme)
	operatorsv1alpha1.AddToScheme(scheme)

	cli := fakeclientset.NewSimpleClientset()
	fakeDiscovery, ok := cli.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		logrus.Error("Error creating fake discovery client")
		os.Exit(1)
	}

	// Create a fake client to mock API calls
	return objs, fakeDiscovery, *scheme
}

func InitCheWithSimpleCR() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			// todo add some spec to check controller ifs like external db, ssl etc
			Server: orgv1.CheClusterSpecServer{
				CheWorkspaceClusterRole: "cluster-admin",
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(false),
			},
		},
	}
}

func InitCheCRWithImagePullerEnabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
		},
	}
}

func InitCheCRWithImagePullerFinalizer() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
		},
	}
}

func ExpectedCheCRWithImagePullerFinalizer() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: "org.eclipse.che/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
			ResourceVersion: "1",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
		},
	}
}

func InitCheCRWithImagePullerDisabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: false,
			},
		},
	}
}

func InitCheCRWithImagePullerEnabledAndDefaultValuesSet() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName: "kubernetes-image-puller",
					ConfigMapName:  "k8s-image-puller",
				},
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(false),
			},
		},
	}
}

func InitCheCRWithImagePullerEnabledAndNewValuesSet() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName: "kubernetes-image-puller-trigger-update",
					ConfigMapName:  "k8s-image-puller-trigger-update",
				},
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(false),
			},
		},
	}
}
