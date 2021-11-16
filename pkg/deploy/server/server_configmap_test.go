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
package server

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
)

func TestNewCheConfigMap(t *testing.T) {
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
			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestConfigMap(t *testing.T) {
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
			name:         "Test k8s data, no tls secret",
			isOpenShift:  false,
			isOpenShift4: false,
			initObjects:  []runtime.Object{},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						WorkspaceNamespaceDefault: "<username>-che",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_KUBERNETES_TLS__CERT": "",
				"CHE_INFRA_KUBERNETES_TLS__KEY":  "",
			},
		},
		{
			name:         "Test k8s data, with tls secret",
			isOpenShift:  false,
			isOpenShift4: false,
			initObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "che-tls",
						Namespace: "eclipse-che",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("CRT"),
						"tls.key": []byte("KEY"),
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						WorkspaceNamespaceDefault: "<username>-che",
					},
					K8s: orgv1.CheClusterSpecK8SOnly{
						TlsSecretName: "che-tls",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_KUBERNETES_TLS__CERT": "CRT",
				"CHE_INFRA_KUBERNETES_TLS__KEY":  "KEY",
			},
		},
		{
			name: "Test k8s data, check public url when internal network enabled.",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						CheHost: "che-host",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_WEBSOCKET_ENDPOINT": "ws://che-host/api/websocket",
			},
		},
		{
			name: "Test k8s data, with internal cluster svc names",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						CheHost: "che-host",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_WEBSOCKET_INTERNAL_ENDPOINT": "ws://che-host.eclipse-che.svc:8080/api/websocket",
			},
		},
		{
			name: "Test k8s data, without internal cluster svc names",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						CheHost:                        "che-host",
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
					},
				},
			},
			expectedData: map[string]string{
				"CHE_WEBSOCKET_ENDPOINT": "ws://che-host/api/websocket",
			},
		},
		{
			name: "Kubernetes strategy should be set correctly",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					K8s: orgv1.CheClusterSpecK8SOnly{
						IngressStrategy: "single-host",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_KUBERNETES_SERVER__STRATEGY": "single-host",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, testCase.initObjects)

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestUpdateBitBucketEndpoints(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *orgv1.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Test set BitBucket endpoints from secret",
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "github-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "bitbucket_endpoint_2",
						},
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_2",
			},
		},
		{
			name: "Test update BitBucket endpoints",
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "github-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "bitbucket_endpoint_2",
						},
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						CustomCheProperties: map[string]string{
							"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_1",
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_1,bitbucket_endpoint_2",
			},
		},
		{
			name:        "Test don't update BitBucket endpoints",
			initObjects: []runtime.Object{},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						CustomCheProperties: map[string]string{
							"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_1",
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_1",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, testCase.initObjects)

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyDevfileRegistryURL(t *testing.T) {
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
			name: "Test devfile registry urls #1",
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
						ExternalDevfileRegistry: true,
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "http://devfile-registry.external.1"},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.external.1",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test devfile registry urls #2",
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
						ExternalDevfileRegistry: true,
						DevfileRegistryUrl:      "http://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "http://devfile-registry.external.2"},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.external.1 http://devfile-registry.external.2",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test devfile registry urls #3",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						ExternalDevfileRegistry:        true,
						DevfileRegistryUrl:             "http://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "http://devfile-registry.external.2"},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.external.1 http://devfile-registry.external.2",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test devfile registry urls #4",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						ExternalDevfileRegistry:        false,
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry.internal",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.internal",
			},
		},
		{
			name: "Test devfile registry urls #5",
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
						ExternalDevfileRegistry: false,
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry.internal",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://devfile-registry.eclipse-che.svc:8080",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.internal",
			},
		},
		{
			name: "Test devfile registry urls #6",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						ExternalDevfileRegistry:        false,
						DevfileRegistryUrl:             "http://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "http://devfile-registry.external.2"},
						},
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry.internal",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.internal http://devfile-registry.external.1 http://devfile-registry.external.2",
			},
		},
		{
			name: "Test devfile registry urls #7",
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
						ExternalDevfileRegistry: false,
						DevfileRegistryUrl:      "http://devfile-registry.external.1",
						ExternalDevfileRegistries: []orgv1.ExternalDevfileRegistries{
							{Url: "http://devfile-registry.external.2"},
						},
					},
				},
				Status: orgv1.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry.internal",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://devfile-registry.eclipse-che.svc:8080",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.internal http://devfile-registry.external.1 http://devfile-registry.external.2",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
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
			name: "Test CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL #1",
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
						ExternalPluginRegistry: true,
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
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL #2",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						ExternalPluginRegistry:         true,
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
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL #3",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						ExternalPluginRegistry:         false,
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
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL #4",
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
						ExternalPluginRegistry: false,
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
			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
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
			name: "Should be an empty when internal network is disabled",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
						CheHost:                        "che-host",
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth: util.NewBoolPointer(false),
					},
				},
			},
			expectedData: map[string]string{
				"CHE_API_INTERNAL": "",
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
						CheHost: "http://che-host",
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
			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
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
			name: "Should be an empty when enabled 'external' public identity provider url and internal network is enabled #1",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: true,
						IdentityProviderURL:      "http://external-keycloak",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "",
				"CHE_KEYCLOAK_AUTH__SERVER__URL":           "http://external-keycloak/auth",
			},
		},
		{
			name: "Should be an empty when enabled 'external' public identity provider url and internal network is enabled #2",
			cheCluster: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: true,
						IdentityProviderURL:      "http://external-keycloak/auth",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "",
				"CHE_KEYCLOAK_AUTH__SERVER__URL":           "http://external-keycloak/auth",
			},
		},
		{
			name: "Should be and empty when enabled 'external' public identity provider url and internal network is disabled",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: true,
						IdentityProviderURL:      "http://external-keycloak",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "",
				"CHE_KEYCLOAK_AUTH__SERVER__URL":           "http://external-keycloak/auth",
			},
		},
		{
			name: "Should be an empty when internal network is disabled",
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
						DisableInternalClusterSVCNames: pointer.BoolPtr(true),
					},
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: false,
						IdentityProviderURL:      "http://keycloak/auth",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "",
				"CHE_KEYCLOAK_AUTH__SERVER__URL":           "http://keycloak/auth",
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
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:           util.NewBoolPointer(false),
						ExternalIdentityProvider: false,
						IdentityProviderURL:      "http://keycloak/auth",
					},
				},
			},
			expectedData: map[string]string{
				"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL": "http://keycloak.eclipse-che.svc:8080/auth",
				"CHE_KEYCLOAK_AUTH__SERVER__URL":           "http://keycloak/auth",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			util.IsOpenShift = testCase.isOpenShift
			util.IsOpenShift4 = testCase.isOpenShift4
			deployContext := deploy.GetTestDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewServer(deployContext)
			actualData, err := server.getCheConfigMapData()
			if err != nil {
				t.Fatalf("Error creating ConfigMap data: %v", err)
			}

			util.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}
