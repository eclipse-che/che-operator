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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

func TestNewCheConfigMap(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *chev2.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name:        "Test",
			initObjects: []runtime.Object{},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_WORKSPACE_NO_PROXY": "myproxy.myhostname.com",
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER": "openshift-v4",
				"CHE_API":                "https://che-host/api",
				"CHE_WORKSPACE_NO_PROXY": "myproxy.myhostname.com",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewCheServerReconciler()
			actualData, err := server.getCheConfigMapData(ctx)
			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestConfigMap(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *chev2.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name:        "Test k8s data, no tls secret",
			initObjects: []runtime.Object{},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultNamespace: chev2.DefaultNamespace{
							Template: "<username>-che",
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INFRA_KUBERNETES_TLS__CERT": "",
				"CHE_INFRA_KUBERNETES_TLS__KEY":  "",
			},
		},
		{
			name: "Test k8s data, with tls secret",
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
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					DevEnvironments: chev2.CheClusterDevEnvironments{
						DefaultNamespace: chev2.DefaultNamespace{
							Template: "<username>-che",
						},
					},
					Networking: chev2.CheClusterSpecNetworking{
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
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
			expectedData: map[string]string{
				"CHE_WEBSOCKET_ENDPOINT": "wss://che-host/api/websocket",
			},
		},
		{
			name: "Test k8s data, with internal cluster svc names",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedData: map[string]string{
				"CHE_WEBSOCKET_INTERNAL_ENDPOINT": "ws://che-host.eclipse-che.svc:8080/api/websocket",
			},
		},
		{
			name: "Test k8s data, without internal cluster svc names",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
			expectedData: map[string]string{
				"CHE_WEBSOCKET_ENDPOINT": "wss://che-host/api/websocket",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, testCase.initObjects)

			server := NewCheServerReconciler()
			actualData, err := server.getCheConfigMapData(ctx)
			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestUpdateIntegrationServerEndpoints(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *chev2.CheCluster
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
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
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
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_1",
							},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_2,bitbucket_endpoint_1",
			},
		},
		{
			name:        "Test don't update BitBucket endpoints",
			initObjects: []runtime.Object{},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS": "bitbucket_endpoint_1",
							},
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
			ctx := test.GetDeployContext(testCase.cheCluster, testCase.initObjects)

			server := NewCheServerReconciler()
			actualData, err := server.getCheConfigMapData(ctx)
			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyDevfileRegistryURL(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *chev2.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Test devfile registry urls #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							DisableInternalRegistry: true,
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
								{Url: "http://devfile-registry.external.1"},
							},
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
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						DevfileRegistry: chev2.DevfileRegistry{
							ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
								{Url: "http://devfile-registry.external.2"},
							},
						},
					},
				},
				Status: chev2.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry.internal.1",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.internal.1 http://devfile-registry.external.2",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://devfile-registry.eclipse-che.svc:8080",
			},
		},
		{
			name: "Test devfile registry urls #5",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					DevfileRegistryURL: "http://devfile-registry.internal",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL": "http://devfile-registry.eclipse-che.svc:8080",
				"CHE_WORKSPACE_DEVFILE__REGISTRY__URL":           "http://devfile-registry.internal",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewCheServerReconciler()
			actualData, err := server.getCheConfigMapData(ctx)
			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyInternalPluginRegistryServiceURL(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *chev2.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Test CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						PluginRegistry: chev2.PluginRegistry{
							DisableInternalRegistry: true,
						},
					},
				},
				Status: chev2.CheClusterStatus{
					PluginRegistryURL: "http://external-plugin-registry",
				},
			},
			expectedData: map[string]string{
				"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL": "",
			},
		},
		{
			name: "Test CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
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
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewCheServerReconciler()
			actualData, err := server.getCheConfigMapData(ctx)
			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestShouldSetUpCorrectlyInternalCheServerURL(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []runtime.Object
		cheCluster   *chev2.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Should use internal che-server url, when internal network is enabled",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Hostname: "che-host",
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
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			server := NewCheServerReconciler()
			actualData, err := server.getCheConfigMapData(ctx)
			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}
