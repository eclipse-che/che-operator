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

package che

import (
	"context"
	"fmt"
	"os"
	"strconv"

	mocks "github.com/eclipse-che/che-operator/mocks"

	"reflect"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/golang/mock/gomock"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	devworkspace "github.com/eclipse-che/che-operator/pkg/deploy/dev-workspace"
	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/google/go-cmp/cmp"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"

	console "github.com/openshift/api/console/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/utils/pointer"

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

	configv1 "github.com/openshift/api/config/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"testing"
)

var (
	namespace = "eclipse-che"
)

func TestNativeUserModeEnabled(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []runtime.Object
		isOpenshift             bool
		devworkspaceEnabled     bool
		initialNativeUserValue  *bool
		expectedNativeUserValue *bool
	}

	testCases := []testCase{
		{
			name:                    "che-operator should use nativeUserMode when devworkspaces on openshift and no initial value in CR for nativeUserMode",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  nil,
			expectedNativeUserValue: pointer.BoolPtr(true),
		},
		{
			name:                    "che-operator should use nativeUserMode value from initial CR",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  pointer.BoolPtr(false),
			expectedNativeUserValue: pointer.BoolPtr(false),
		},
		{
			name:                    "che-operator should use nativeUserMode value from initial CR",
			isOpenshift:             true,
			devworkspaceEnabled:     true,
			initialNativeUserValue:  pointer.BoolPtr(true),
			expectedNativeUserValue: pointer.BoolPtr(true),
		},
		//{
		//	name:                    "che-operator should not modify nativeUserMode when not on openshift",
		//	isOpenshift:             false,
		//	devworkspaceEnabled:     true,
		//	initialNativeUserValue:  nil,
		//	expectedNativeUserValue: nil,
		//},
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
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(routev1.GroupVersion, &routev1.Route{})
			scheme.AddKnownTypes(oauthv1.SchemeGroupVersion, &oauthv1.OAuthClient{})
			scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.Proxy{})
			scheme.AddKnownTypes(crdv1.SchemeGroupVersion, &crdv1.CustomResourceDefinition{})

			initCR := InitCheWithSimpleCR().DeepCopy()
			initCR.Spec.DevWorkspace.Enable = testCase.devworkspaceEnabled
			initCR.Spec.Auth.NativeUserMode = testCase.initialNativeUserValue
			testCase.initObjects = append(testCase.initObjects, initCR)

			util.IsOpenShift = testCase.isOpenshift

			// reread templates (workaround after setting IsOpenShift value)
			devworkspace.DevWorkspaceTemplates = devworkspace.DevWorkspaceTemplatesPath()
			devworkspace.DevWorkspaceIssuerFile = devworkspace.DevWorkspaceTemplates + "/devworkspace-controller-selfsigned-issuer.Issuer.yaml"
			devworkspace.DevWorkspaceCertificateFile = devworkspace.DevWorkspaceTemplates + "/devworkspace-controller-serving-cert.Certificate.yaml"

			cli := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{}

			if !ok {
				t.Fatal("Error creating fake discovery client")
			}

			r := NewReconciler(cli, nonCachedClient, fakeDiscovery, scheme, "")
			r.tests = true

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			_, err := r.Reconcile(context.TODO(), req)
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
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

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

			r := NewReconciler(cli, nonCachedClient, fakeDiscovery, scheme, "")
			r.tests = true

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			util.IsOpenShift = true
			util.IsOpenShift4 = false
			_, err := r.Reconcile(context.TODO(), req)
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
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

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

			r := NewReconciler(cli, nonCachedClient, fakeDiscovery, scheme, "")
			r.tests = true

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			_, err := r.Reconcile(context.TODO(), req)
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

func TestCheController(t *testing.T) {
	var err error

	util.IsOpenShift = true
	util.IsOpenShift4 = false

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := NewReconciler(cl, cl, dc, &scheme, "")
	r.tests = true

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

	reconcileLoops := 4
	for i := 0; i < reconcileLoops; i++ {
		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Fatalf("reconcile: (%v)", err)
		}
	}

	// get devfile-registry configmap
	devfilecm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DevfileRegistryName, Namespace: cheCR.Namespace}, devfilecm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", devfilecm.Name, err)
	}

	// get plugin-registry configmap
	pluginRegistrycm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DevfileRegistryName, Namespace: cheCR.Namespace}, pluginRegistrycm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", pluginRegistrycm.Name, err)
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
	for i := 0; i < reconcileLoops; i++ {
		_, err = r.Reconcile(context.TODO(), req)
		if err != nil {
			t.Fatalf("reconcile: (%v)", err)
		}
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
		// If the test fails here without obvious reason, it could mean that there was not enought reconcile loops before.
		// To fix the above problem, just increase reconcileLoops variable above.
		t.Errorf("ConfigMap wasn't updated. Expecting true, but got: %s", cm.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"])
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

	_, err = r.Reconcile(context.TODO(), req)
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
		Client:           r.client,
		NonCachingClient: r.client,
		Scheme:           r.Scheme,
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
	oAuthClient := &oauthv1.OAuthClient{}
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
	_, err = r.Reconcile(context.TODO(), req)
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
	_, err = r.Reconcile(context.TODO(), req)
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
	oauthClient := &oauthv1.OAuthClient{}
	err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName}, oauthClient)
	if err == nil {
		t.Fatalf("OauthClient %s has not been deleted", oauthClientName)
	}
	logrus.Infof("Disregard the error above. OauthClient %s has been deleted", oauthClientName)
}

func TestConfiguringLabelsForRoutes(t *testing.T) {
	util.IsOpenShift = true
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := NewReconciler(cl, cl, dc, &scheme, "")
	r.tests = true

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
	_, err := r.Reconcile(context.TODO(), req)
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
	_, err = r.Reconcile(context.TODO(), req)
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
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			scheme := scheme.Scheme
			orgv1.SchemeBuilder.AddToScheme(scheme)
			scheme.AddKnownTypes(oauthv1.SchemeGroupVersion, &oauthv1.OAuthClient{})
			scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
			scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.OAuth{}, &configv1.Proxy{})
			scheme.AddKnownTypes(routev1.GroupVersion, &routev1.Route{})

			initCR := testCase.checluster
			initCR.Spec.Auth.OpenShiftoAuth = pointer.BoolPtr(false)
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

			r := NewReconciler(cli, nonCachedClient, fakeDiscovery, scheme, "")
			r.tests = true

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: namespace,
				},
			}

			_, err := r.Reconcile(context.TODO(), req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}
			_, err = r.Reconcile(context.TODO(), req)
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

	oAuthClient := &oauthv1.OAuthClient{}
	users := &userv1.UserList{}
	user := &userv1.User{}

	// Register operator types with the runtime scheme
	scheme.AddKnownTypes(oauthv1.SchemeGroupVersion, oAuthClient)
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
				OpenShiftoAuth: pointer.BoolPtr(false),
			},
		},
	}
}
