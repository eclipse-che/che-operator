//
// Copyright (c) 2012-2021 Red Hat, Inc.
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
	"strconv"
	"strings"
	"unicode/utf8"

	mocks "github.com/eclipse-che/che-operator/mocks"

	"reflect"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
	"github.com/golang/mock/gomock"

	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"

	console "github.com/openshift/api/console/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
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

	che_mocks "github.com/eclipse-che/che-operator/mocks/controllers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/yaml"

	"testing"
)

var (
	namespace       = "eclipse-che"
	csvName         = "kubernetes-imagepuller-operator.v0.0.9"
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
	valueTrue             = true
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
	oAuthWithNoIdentityProviders = &configv1.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			// Namespace: namespace,
		},
	}
	oAuthWithIdentityProvider = &configv1.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			// Namespace: namespace,
		},
		Spec: configv1.OAuthSpec{
			IdentityProviders: []configv1.IdentityProvider{
				{
					Name: "htpasswd",
				},
			},
		},
	}
	route = &routev1.Route{}
	proxy = &configv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}
	defaultImagePullerImages string
)

func init() {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile("../../config/manager/manager.yaml")
	yaml.Unmarshal(data, operator)
	if err == nil {
		for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
			os.Setenv(env.Name, env.Value)
		}
	}
	defaultImagePullerImages = "che-workspace-plugin-broker-metadata=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata") +
		";che-workspace-plugin-broker-artifacts=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts") + ";"
}

func TestNativeUserModeEnabled(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []runtime.Object
		isOpenshift             bool
		devworkspaceEnabled     bool
		initialNativeUserValue  *bool
		expectedNativeUserValue *bool
		mockFunction            func(ctrl *gomock.Controller, crNamespace string, usernamePrefix string) *che_mocks.MockOpenShiftOAuthUserHandler
	}

	testCases := []testCase{
		{
			name:                    "che-operator should use nativeUserMode when devworkspaces on openshift and no initial value in CR for nativeUserMode",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: util.NewBoolPointer(true),
		},
		{
			name:                    "che-operator should use nativeUserMode value from initial CR",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  util.NewBoolPointer(false),
			expectedNativeUserValue: util.NewBoolPointer(false),
		},
		{
			name:                    "che-operator should use nativeUserMode value from initial CR",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  util.NewBoolPointer(true),
			expectedNativeUserValue: util.NewBoolPointer(true),
		},
		{
			name:                    "che-operator should not modify nativeUserMode when not on openshift",
			isOpenshift:             false,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: nil,
		},
		{
			name:                    "che-operator not modify nativeUserMode when devworkspace not enabled",
			isOpenshift:             true,
			devworkspaceEnabled:     false,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(routev1.GroupVersion, route)
			scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
			initCR := InitCheWithSimpleCR().DeepCopy()
			testCase.initObjects = append(testCase.initObjects, initCR)

			initCR.Spec.DevWorkspace.Enable = testCase.devworkspaceEnabled
			initCR.Spec.Auth.NativeUserMode = testCase.initialNativeUserValue
			util.IsOpenShift = testCase.isOpenshift

			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			if !ok {
				t.Fatal("Error creating fake discovery client")
			}

			r := &CheClusterReconciler{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				Scheme:          scheme,
				tests:           true,
				Log:             ctrl.Log.WithName("controllers").WithName("CheCluster"),
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}
			cr := &orgv1.CheCluster{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cr); err != nil {
				t.Errorf("CR not found")
			}

			if !reflect.DeepEqual(testCase.expectedNativeUserValue, cr.Spec.Auth.NativeUserMode) {
				expectedValue, actualValue := "nil", "nil"
				if testCase.expectedNativeUserValue != nil {
					expectedValue = strconv.FormatBool(*testCase.expectedNativeUserValue)
				}
				if cr.Spec.Auth.NativeUserMode != nil {
					actualValue = strconv.FormatBool(*cr.Spec.Auth.NativeUserMode)
				}

				t.Errorf("Expected nativeUserMode '%+v', but found '%+v' for input '%+v'",
					expectedValue, actualValue, testCase)
			}
		})
	}
}

func TestCaseAutoDetectOAuth(t *testing.T) {
	type testCase struct {
		name                                string
		initObjects                         []runtime.Object
		isOpenshift3                        bool
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
			isOpenshift3:      true,
			initialOAuthValue: nil,
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should auto disable oAuth when Che CR with nil oAuth on the Openshift 3 with no users",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauth.OAuthClient{},
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = true even if there no users on the Openshift 3",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauth.OAuthClient{},
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(true),
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there are some users on the Openshift 3",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauth.OAuthClient{},
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(true),
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = false even if there are some users on the Openshift 3",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauth.OAuthClient{},
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = false even if no users on the Openshift 3",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauth.OAuthClient{},
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should auto enable oAuth when Che CR with nil value on the Openshift 4 with identity providers",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
				proxy,
			},
			isOpenshift3:      false,
			initialOAuthValue: nil,
			oAuthExpected:     util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
				proxy,
			},
			isOpenshift3:                     false,
			initialOAuthValue:                util.NewBoolPointer(true),
			oAuthExpected:                    util.NewBoolPointer(true),
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(true),
		},
		{
			name: "che-operator should not create initial user and enable oAuth, when oAuth = true, initialOpenShiftOAuthUserEnabled = true and there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
				proxy,
			},
			isOpenshift3:                     false,
			initialOAuthValue:                util.NewBoolPointer(true),
			oAuthExpected:                    util.NewBoolPointer(true),
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = true even if there are some users on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
				proxy,
			},
			isOpenshift3:                     true,
			initialOAuthValue:                util.NewBoolPointer(true),
			oAuthExpected:                    util.NewBoolPointer(true),
			initialOpenShiftOAuthUserEnabled: util.NewBoolPointer(true),
		},
		{
			name: "che-operator should respect oAuth = false even if there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
				proxy,
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name: "che-operator should respect oAuth = false even if there are some users on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
				proxy,
			},
			isOpenshift3:      true,
			initialOAuthValue: util.NewBoolPointer(false),
			oAuthExpected:     util.NewBoolPointer(false),
		},
		{
			name:                             "che-operator should auto disable oAuth on error retieve identity providers",
			initObjects:                      []runtime.Object{},
			isOpenshift3:                     true,
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
			scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.OAuth{}, &configv1.Proxy{})
			scheme.AddKnownTypes(routev1.GroupVersion, route)
			scheme.AddKnownTypes(configv1.GroupVersion, &configv1.Proxy{})
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

			r := &CheClusterReconciler{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				Scheme:          scheme,
				tests:           true,
				userHandler:     userHandlerMock,
				Log:             ctrl.Log.WithName("controllers").WithName("CheCluster"),
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			util.IsOpenShift = true
			util.IsOpenShift4 = !testCase.isOpenshift3

			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}

			cheCR := &orgv1.CheCluster{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cheCR); err != nil {
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

func TestEnsureServerExposureStrategy(t *testing.T) {
	type testCase struct {
		name                string
		expectedCr          *orgv1.CheCluster
		devWorkspaceEnabled bool
		initObjects         []runtime.Object
	}

	testCases := []testCase{
		{
			name: "Single Host should be enabled if devWorkspace is enabled",
			expectedCr: &orgv1.CheCluster{
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "single-host",
					},
				},
			},
			devWorkspaceEnabled: true,
		},
		{
			name: "Multi Host should be enabled if devWorkspace is not enabled",
			expectedCr: &orgv1.CheCluster{
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
					},
				},
			},
			devWorkspaceEnabled: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			initCR := InitCheWithSimpleCR().DeepCopy()
			testCase.initObjects = append(testCase.initObjects, initCR)
			if testCase.devWorkspaceEnabled {
				initCR.Spec.DevWorkspace.Enable = true
			}
			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			if !ok {
				t.Fatal("Error creating fake discovery client")
			}

			r := &CheClusterReconciler{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				Scheme:          scheme,
				tests:           true,
				Log:             ctrl.Log.WithName("controllers").WithName("CheCluster"),
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}
			cr := &orgv1.CheCluster{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cr); err != nil {
				t.Errorf("CR not found")
			}
			if !reflect.DeepEqual(testCase.expectedCr.Spec.Server.ServerExposureStrategy, cr.Spec.Server.ServerExposureStrategy) {
				t.Errorf("Expected CR and CR returned from API server are different (-want +got): %v", cmp.Diff(testCase.expectedCr.Spec.Server.ServerExposureStrategy, cr.Spec.Server.ServerExposureStrategy))
			}
		})
	}
}

func TestShouldSetUpCorrectlyDevfileRegistryURL(t *testing.T) {
	type testCase struct {
		name                       string
		isOpenShift                bool
		isOpenShift4               bool
		initObjects                []runtime.Object
		cheCluster                 *orgv1.CheCluster
		expectedDevfileRegistryURL string
	}

	testCases := []testCase{
		{
			name: "Test Status.DevfileRegistryURL #1",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      os.Getenv("CHE_FLAVOR"),
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ExternalDevfileRegistry: false,
					},
				},
			},
			expectedDevfileRegistryURL: "http://devfile-registry-eclipse-che./",
		},
		{
			name: "Test Status.DevfileRegistryURL #2",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      os.Getenv("CHE_FLAVOR"),
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ExternalDevfileRegistry: false,
						DevfileRegistryUrl:      "https://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "https://devfile-registry.external.2"},
						},
					},
				},
			},
			expectedDevfileRegistryURL: "http://devfile-registry-eclipse-che./",
		},
		{
			name: "Test Status.DevfileRegistryURL #2",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      os.Getenv("CHE_FLAVOR"),
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ExternalDevfileRegistry: true,
						DevfileRegistryUrl:      "https://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "https://devfile-registry.external.2"},
						},
					},
				},
			},
			expectedDevfileRegistryURL: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.cheCluster)
			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			if !ok {
				t.Fatal("Error creating fake discovery client")
			}
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			r := &CheClusterReconciler{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				Scheme:          scheme,
				tests:           true,
				Log:             ctrl.Log.WithName("controllers").WithName("CheCluster"),
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}

			cr := &orgv1.CheCluster{}
			if err := r.client.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cr); err != nil {
				t.Errorf("CR not found")
			}

			if cr.Status.DevfileRegistryURL != testCase.expectedDevfileRegistryURL {
				t.Fatalf("Exected: %s, but found: %s", testCase.expectedDevfileRegistryURL, cr.Status.DevfileRegistryURL)
			}
		})
	}
}

func TestImagePullerConfiguration(t *testing.T) {
	oldBrokerMetaDataImage := strings.Split(os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata"), ":")[0] + ":old"
	oldBrokerArtifactsImage := strings.Split(os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts"), ":")[0] + ":old"
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
					Name:            os.Getenv("CHE_FLAVOR"),
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
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
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
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, user images set, subscription exists, should create a KubernetesImagePuller with user images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("image=image_url"),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url", ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, one default image set, subscription exists, should update KubernetesImagePuller default image",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";"),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				InitImagePuller(ImagePullerOptions{SpecImages: "che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "che-workspace-plugin-broker-metadata=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata") + ";", ObjectMetaResourceVersion: "2"}),
		},
		{
			name:   "image puller enabled, one default image set, subscription exists, should update KubernetesImagePuller default images while keeping user image",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("image=image_url;che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";"),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;che-workspace-plugin-broker-metadata=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata") + ";", ObjectMetaResourceVersion: "2"}),
		},
		{
			name:   "image puller enabled, default images set, subscription exists, should update KubernetesImagePuller default images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";che-workspace-plugin-broker-artifacts=" + oldBrokerArtifactsImage + ";"),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				InitImagePuller(ImagePullerOptions{SpecImages: "che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";che-workspace-plugin-broker-artifacts=" + oldBrokerArtifactsImage + ";", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "2"}),
		},
		{
			name:   "image puller enabled, latest default images set, subscription exists, should not update KubernetesImagePuller default images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet(defaultImagePullerImages),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, default images not set, subscription exists, should not set KubernetesImagePuller default images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("image=image_url;"),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;", ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, KubernetesImagePuller created and spec in CheCluster is different, should update the KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndNewValuesSet(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				getDefaultImagePuller(),
			},
			expectedImagePuller: &chev1alpha1.KubernetesImagePuller{
				TypeMeta: metav1.TypeMeta{Kind: "KubernetesImagePuller", APIVersion: "che.eclipse.org/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "2",
					Name:            os.Getenv("CHE_FLAVOR") + "-image-puller",
					Namespace:       namespace,
					Labels: map[string]string{
						"app":                       "che",
						"component":                 "kubernetes-image-puller",
						"app.kubernetes.io/part-of": os.Getenv("CHE_FLAVOR"),
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "org.eclipse.che/v1",
							Kind:               "CheCluster",
							BlockOwnerDeletion: &valueTrue,
							Controller:         &valueTrue,
							Name:               os.Getenv("CHE_FLAVOR"),
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
				getDefaultImagePuller(),
			},
			shouldDelete: true,
		},
		{
			name:   "image puller already created, finalizer deleted",
			initCR: InitCheCRWithImagePullerFinalizerAndDeletionTimestamp(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				clusterServiceVersion,
				getDefaultImagePuller(),
			},
			expectedCR: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:              os.Getenv("CHE_FLAVOR"),
					Namespace:         namespace,
					ResourceVersion:   "1",
					DeletionTimestamp: &metav1.Time{Time: time.Unix(1, 0)},
				},
				Spec: orgv1.CheClusterSpec{
					ImagePuller: orgv1.CheClusterSpecImagePuller{
						Enable: true,
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
					},
				},
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

			r := &CheClusterReconciler{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				Scheme:          scheme.Scheme,
				tests:           true,
				Log:             ctrl.Log.WithName("controllers").WithName("CheCluster"),
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
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
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: os.Getenv("CHE_FLAVOR")}, gotCR)
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

				diff := cmp.Diff(testCase.expectedImagePuller, gotImagePuller, cmpopts.IgnoreFields(chev1alpha1.KubernetesImagePullerSpec{}, "Images"))
				if diff != "" {
					t.Errorf("Expected KubernetesImagePuller and KubernetesImagePuller returned from API server differ (-want, +got): %v", diff)
				}

				expectedImages := nonEmptySplit(testCase.expectedImagePuller.Spec.Images, ";")
				if len(nonEmptySplit(testCase.expectedImagePuller.Spec.Images, ";")) != len(expectedImages) {
					t.Errorf("Expected KubernetesImagePuller returns %d images", len(expectedImages))
				}

				for _, expectedImage := range expectedImages {
					if !strings.Contains(gotImagePuller.Spec.Images, expectedImage) {
						t.Errorf("Expected KubernetesImagePuller returned image: %s, but it did not", expectedImage)
					}
				}
			}
			if testCase.shouldDelete {

				imagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: os.Getenv("CHE_FLAVOR") + "-image-puller"}, imagePuller)
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
					t.Fatalf("Should not have found Subscription: %v", err)
				}

				operatorGroup := &operatorsv1.OperatorGroup{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, operatorGroup)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found OperatorGroup: %v", err)
				}
			}
		})
	}
}

func TestCheController(t *testing.T) {
	util.IsOpenShift = true
	util.IsOpenShift4 = false

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &CheClusterReconciler{client: cl, nonCachedClient: cl, Scheme: &scheme, discoveryClient: dc, tests: true, Log: ctrl.Log.WithName("controllers").WithName("CheCluster")}

	// get CR
	cheCR := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheHost: "eclipse.org",
			},
		},
	}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      os.Getenv("CHE_FLAVOR"),
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

	// get CR
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
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

	// get CR
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
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
		Scheme:          r.Scheme,
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

	// Get CheCR one more time to get it with newer Che url in the status.
	r.client.Get(context.TODO(), types.NamespacedName{Name: cheCR.GetName(), Namespace: cheCR.GetNamespace()}, cheCR)
	if err != nil {
		t.Fatalf("Failed to get custom resource Eclipse Che: %s", err.Error())
	}
	if cheCR.Status.CheURL != "https://eclipse.org" {
		t.Fatalf("Expected che host url in the custom resource status: %s, but got %s", "https://eclipse.org", cheCR.Status.CheURL)
	}

	// check if oAuthClient is deleted after CR is deleted (finalizer logic)
	// since fake api does not set deletion timestamp, CR is updated in tests rather than deleted
	logrus.Info("Updating CR with deletion timestamp")
	deletionTimestamp := &metav1.Time{Time: time.Now()}
	cheCR.DeletionTimestamp = deletionTimestamp
	if err := r.client.Update(context.TODO(), cheCR); err != nil {
		t.Fatalf("Failed to update CR: %s", err)
	}
	if err := deploy.ReconcileOAuthClientFinalizer(deployContext); err != nil {
		t.Fatal("Failed to reconcile oAuthClient")
	}
	oauthClientName := cheCR.Spec.Auth.OAuthClientName
	oauthClient := &oauth.OAuthClient{}
	err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName}, oauthClient)
	if err == nil {
		t.Fatalf("OauthClient %s has not been deleted", oauthClientName)
	}
	logrus.Infof("Disregard the error above. OauthClient %s has been deleted", oauthClientName)
}

func TestConfiguringLabelsForRoutes(t *testing.T) {
	util.IsOpenShift = true
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &CheClusterReconciler{client: cl, nonCachedClient: cl, Scheme: &scheme, discoveryClient: dc, tests: true, Log: ctrl.Log.WithName("controllers").WithName("CheCluster")}

	// get CR
	cheCR := &orgv1.CheCluster{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
		},
	}

	// reconcile
	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
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
	util.IsOpenShift = true

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
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
			scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
			scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.OAuth{}, &configv1.Proxy{})
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

			r := &CheClusterReconciler{
				client:            cli,
				nonCachedClient:   nonCachedClient,
				discoveryClient:   fakeDiscovery,
				Scheme:            scheme,
				permissionChecker: m,
				tests:             true,
				Log:               ctrl.Log.WithName("controllers").WithName("CheCluster"),
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
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
				manageNamespacesClusterRoleName := fmt.Sprintf(CheNamespaceEditorClusterRoleNameTemplate, namespace)
				cheManageNamespaceClusterRole := &rbac.ClusterRole{}
				if err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: manageNamespacesClusterRoleName}, cheManageNamespaceClusterRole); err != nil {
					t.Errorf("role '%s' not found", manageNamespacesClusterRoleName)
				}
				cheManageNamespaceClusterRoleBinding := &rbac.ClusterRoleBinding{}
				if err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: manageNamespacesClusterRoleName}, cheManageNamespaceClusterRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", manageNamespacesClusterRoleName)
				}

				cheWorkspacesClusterRoleName := fmt.Sprintf(CheWorkspacesClusterRoleNameTemplate, namespace)
				cheWorkspacesClusterRole := &rbac.ClusterRole{}
				if err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: cheWorkspacesClusterRoleName}, cheWorkspacesClusterRole); err != nil {
					t.Errorf("role '%s' not found", cheWorkspacesClusterRole)
				}
				cheWorkspacesClusterRoleBinding := &rbac.ClusterRoleBinding{}
				if err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: cheWorkspacesClusterRoleName}, cheWorkspacesClusterRoleBinding); err != nil {
					t.Errorf("rolebinding '%s' not found", cheWorkspacesClusterRole)
				}
			}
		})
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
	scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.Proxy{})

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
	scheme.AddKnownTypes(orgv1.GroupVersion, cheCR)
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
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
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
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func InitCheCRWithImagePullerFinalizer() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func InitCheCRWithImagePullerFinalizerAndDeletionTimestamp() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
			DeletionTimestamp: &metav1.Time{Time: time.Unix(1, 0)},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
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
			Name:      os.Getenv("CHE_FLAVOR"),
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
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func InitCheCRWithImagePullerDisabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
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
			Name:      os.Getenv("CHE_FLAVOR"),
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

func InitCheCRWithImagePullerEnabledAndImagesSet(images string) *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
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
					Images:         images,
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
			Name:      os.Getenv("CHE_FLAVOR"),
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

type ImagePullerOptions struct {
	SpecImages                string
	ObjectMetaResourceVersion string
}

func InitImagePuller(options ImagePullerOptions) *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "che.eclipse.org/v1alpha1",
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR") + "-image-puller",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": os.Getenv("CHE_FLAVOR"),
				"app":                       "che",
				"component":                 "kubernetes-image-puller",
			},
			ResourceVersion: options.ObjectMetaResourceVersion,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "org.eclipse.che/v1",
					Kind:               "CheCluster",
					Controller:         &valueTrue,
					BlockOwnerDeletion: &valueTrue,
					Name:               os.Getenv("CHE_FLAVOR"),
				},
			},
		},
		Spec: chev1alpha1.KubernetesImagePullerSpec{
			DeploymentName: "kubernetes-image-puller",
			ConfigMapName:  "k8s-image-puller",
			Images:         options.SpecImages,
		},
	}
}

func getDefaultImagePuller() *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "che.eclipse.org/v1alpha1",
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR") + "-image-puller",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": os.Getenv("CHE_FLAVOR"),
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
					Name:               os.Getenv("CHE_FLAVOR"),
				},
			},
		},
		Spec: chev1alpha1.KubernetesImagePullerSpec{
			DeploymentName: "kubernetes-image-puller",
			ConfigMapName:  "k8s-image-puller",
			Images:         defaultImagePullerImages,
		},
	}
}

// Split string by separator without empty elems
func nonEmptySplit(lineToSplit string, separator string) []string {
	splitFn := func(c rune) bool {
		runeChar, _ := utf8.DecodeRuneInString(separator)
		return c == runeChar
	}
	return strings.FieldsFunc(lineToSplit, splitFn)
}
