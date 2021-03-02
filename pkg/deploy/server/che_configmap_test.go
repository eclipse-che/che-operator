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
package server

import (
	"os"
	"testing"

	"github.com/eclipse/che-operator/pkg/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
)

func TestCheConfigMap(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		isOpenShift4 bool
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name:         "Test",
			initObjects:  []runtime.Object{},
			isOpenShift:  true,
			isOpenShift4: true,
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						CheHost:    "myhostname.com",
						TlsSupport: true,
						CustomCheProperties: map[string]string{
							"CHE_WORKSPACE_NO_PROXY": "myproxy.myhostname.com",
						},
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(true),
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER": "openshift-v4",
				"CHE_API":                "https://myhostname.com/api",
				"CHE_WORKSPACE_NO_PROXY": "myproxy.myhostname.com",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:          cli,
					NonCachedClient: nonCachedClient,
					Scheme:          scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			actualData, err := GetCheConfigMapData(deployContext)
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyInternalDevfileRegistryServiceURL(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		isOpenShift4 bool
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Should use 'external' devfile registry url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
						ExternalDevfileRegistry:    true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://external-devfile-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://external-devfile-registry",
			},
		},
		{
			name: "Should use 'external' devfile registry url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
						ExternalDevfileRegistry:    true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://external-devfile-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://external-devfile-registry",
			},
		},
		{
			name: "Should use public devfile registry url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
						ExternalDevfileRegistry:    false,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://devfile-registry",
			},
		},
		{
			name: "Should use internal devfile registry url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
						ExternalDevfileRegistry:    false,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://external-devfile-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://devfile-registry.eclipse-che.svc:8080",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:          cli,
					NonCachedClient: nonCachedClient,
					Scheme:          scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
				InternalService: deploy.InternalService{
					DevfileRegistryHost: "http://devfile-registry.eclipse-che.svc:8080",
				},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			actualData, err := GetCheConfigMapData(deployContext)
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyInternalPluginRegistryServiceURL(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		isOpenShift4 bool
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Should use 'external' public plugin registry url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
						ExternalPluginRegistry:     true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					PluginRegistryURL: "http://external-plugin-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "http://external-plugin-registry",
			},
		},
		{
			name: "Should use 'external' public plugin registry url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
						ExternalPluginRegistry:     true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					PluginRegistryURL: "http://external-plugin-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "http://external-plugin-registry",
			},
		},
		{
			name: "Should use public plugin registry url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
						ExternalPluginRegistry:     false,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					PluginRegistryURL: "http://plugin-registry/v3",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "http://plugin-registry/v3",
			},
		},
		{
			name: "Should use internal plugin registry url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
						ExternalPluginRegistry:     false,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
				Status: orgv1.CheClusterStatus{
					PluginRegistryURL: "http://external-plugin-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "http://plugin-registry.eclipse-che.svc:8080/v3",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:          cli,
					NonCachedClient: nonCachedClient,
					Scheme:          scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
				InternalService: deploy.InternalService{
					PluginRegistryHost: "http://plugin-registry.eclipse-che.svc:8080/v3",
				},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			actualData, err := GetCheConfigMapData(deployContext)
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyInternalCheServerURL(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		isOpenShift4 bool
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Should use public che-server url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
						CheHost:                    "che-host",
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
			},
			expectedData: map[string]string{
				"CHE_API_INTERNAL": "http://che-host/api",
			},
		},
		{
			name: "Should use internal che-server url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
						CheHost:                    "http://che-host",
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
			},
			expectedData: map[string]string{
				"CHE_API_INTERNAL": "http://che-host.eclipse-che.svc:8080/api",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:          cli,
					NonCachedClient: nonCachedClient,
					Scheme:          scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
				InternalService: deploy.InternalService{
					CheHost: "http://che-host.eclipse-che.svc:8080",
				},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			actualData, err := GetCheConfigMapData(deployContext)
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyInternalIdentityProviderServiceURL(t *testing.T) {
	type testCase struct {
		name         string
		isOpenShift  bool
		isOpenShift4 bool
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Should use 'external' public identity provider url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: true,
						IdentityProviderURL:      "http://external-keycloak",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "http://external-keycloak/auth",
			},
		},
		{
			name: "Should use 'external' public identity provider url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: true,
						IdentityProviderURL:      "http://external-keycloak",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "http://external-keycloak/auth",
			},
		},
		{
			name: "Should use public identity provider url, when internal network is disabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: false,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: false,
						IdentityProviderURL:      "http://keycloak",
					},
				},
				Status: orgv1.CheClusterStatus{
					KeycloakURL: "http://keycloak",
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "http://keycloak/auth",
			},
		},
		{
			name: "Should use internal identity provider url, when internal network is enabled",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						UseInternalClusterSVCNames: true,
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: false,
						IdentityProviderURL:      "http://keycloak",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "http://keycloak.eclipse-che.svc:8080/auth",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client:          cli,
					NonCachedClient: nonCachedClient,
					Scheme:          scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
				InternalService: deploy.InternalService{
					KeycloakHost: "http://keycloak.eclipse-che.svc:8080",
				},
			}

			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4

			actualData, err := GetCheConfigMapData(deployContext)
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}
