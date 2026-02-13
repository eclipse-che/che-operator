//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
)

func TestGetConfigMapData(t *testing.T) {
	type testCase struct {
		name         string
		cheCluster   *chev2.CheCluster
		initObjects  []runtime.Object
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name:        "Test defaults",
			initObjects: []runtime.Object{},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"EXTRA_PROPERTY": "extra-value",
							},
							Proxy: &chev2.Proxy{
								Url:  "http://127.0.0.1",
								Port: "8080",
							},
						},
					},
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							IdentityProviderURL: "http://identity-provider",
						},
					},
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che.che",
				},
			},
			expectedData: map[string]string{
				"EXTRA_PROPERTY":            "extra-value",
				"JAVA_OPTS":                 "-XX:MaxRAMPercentage=85.0 -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=8080 -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=8080 -Dhttp.nonProxyHosts=''",
				"CHE_HOST":                  "che.che",
				"CHE_PORT":                  "8080",
				"CHE_DEBUG_SERVER":          "false",
				"CHE_LOG_LEVEL":             "INFO",
				"CHE_METRICS_ENABLED":       "false",
				"CHE_INFRASTRUCTURE_ACTIVE": "openshift",
				"CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES":        "eclipse-che-cheworkspaces-clusterrole,eclipse-che-cheworkspaces-devworkspace-clusterrole",
				"CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT":           "<username>-" + defaults.GetCheFlavor(),
				"CHE_INFRA_KUBERNETES_NAMESPACE_CREATION__ALLOWED": "true",
				"KUBERNETES_LABELS":                                labels.FormatLabels(deploy.GetLabels(defaults.GetCheFlavor())),
				"HTTP2_DISABLE":                                    "true",
				"CHE_OIDC_AUTH__SERVER__URL":                       "http://identity-provider",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).Build()
			ctx.Proxy.HttpProxy = "http://127.0.0.1:8080"
			ctx.Proxy.HttpHost = "127.0.0.1"
			ctx.Proxy.HttpPort = "8080"
			ctx.Proxy.HttpsProxy = "http://127.0.0.1:8080"
			ctx.Proxy.HttpsHost = "127.0.0.1"
			ctx.Proxy.HttpsPort = "8080"

			serverReconciler := NewCheServerReconciler()
			actualData, err := serverReconciler.getConfigMapData(ctx)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedData, actualData)
		})
	}
}

func TestGetConfigMapDataWithServerEndpoints(t *testing.T) {
	type testCase struct {
		name         string
		initObjects  []client.Object
		cheCluster   *chev2.CheCluster
		expectedData map[string]string
	}

	testCases := []testCase{
		{
			name: "Test use endpoint from secret",
			initObjects: []client.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bitbucket-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "bitbucket_endpoint",
						},
					},
				},
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "azure-devops-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "azure-devops",
							"che.eclipse.org/scm-server-endpoint": "azure-devops_endpoint",
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
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint",
				"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint",
			},
		},
		{
			name:        "Test use endpoint from extra properties",
			initObjects: []client.Object{},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint",
								"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint",
							},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint",
				"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint",
			},
		},
		{
			name: "Test duplicate endpoints",
			initObjects: []client.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bitbucket-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "bitbucket_endpoint",
						},
					},
				},
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "azure-devops-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "azure-devops",
							"che.eclipse.org/scm-server-endpoint": "azure-devops_endpoint",
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
								"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint",
								"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint",
							},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint",
				"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint",
			},
		},
		{
			name: "Test update endpoints",
			initObjects: []client.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bitbucket-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "bitbucket_endpoint_1",
						},
					},
				},
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "azure-devops-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "azure-devops",
							"che.eclipse.org/scm-server-endpoint": "azure-devops_endpoint_1",
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
								"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint_2",
								"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint_2",
							},
						},
					},
				},
			},
			expectedData: map[string]string{
				"CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS":    "bitbucket_endpoint_1,bitbucket_endpoint_2",
				"CHE_INTEGRATION_AZURE_DEVOPS_SERVER__ENDPOINTS": "azure-devops_endpoint_1,azure-devops_endpoint_2",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.initObjects...).Build()
			serverReconciler := NewCheServerReconciler()

			actualData, err := serverReconciler.getConfigMapData(ctx)

			assert.Nil(t, err)
			test.ValidateContainData(actualData, testCase.expectedData, t)
		})
	}
}

func TestGetConfigMapDataWithUserClusterRoles(t *testing.T) {
	type testCase struct {
		name                     string
		cheCluster               *chev2.CheCluster
		expectedUserClusterRoles string
	}

	testCases := []testCase{
		{
			name: "Test defaults roles",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedUserClusterRoles: "eclipse-che-cheworkspaces-clusterrole,eclipse-che-cheworkspaces-devworkspace-clusterrole",
		},
		{
			name: "Test additional roles #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES": "eclipse-che-cheworkspaces-clusterrole, test-roles-1, test-roles-2",
							},
						},
					},
				},
			},
			expectedUserClusterRoles: "eclipse-che-cheworkspaces-clusterrole,eclipse-che-cheworkspaces-devworkspace-clusterrole,test-roles-1,test-roles-2",
		},
		{
			name: "Test additional roles #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							ExtraProperties: map[string]string{
								"CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES": "eclipse-che-cheworkspaces-clusterrole, test-roles-1, test-roles-2",
							},
						},
					},
					DevEnvironments: chev2.CheClusterDevEnvironments{
						User: &chev2.UserConfiguration{
							ClusterRoles: []string{
								"test-roles-3",
							},
						},
					},
				},
			},
			expectedUserClusterRoles: "eclipse-che-cheworkspaces-clusterrole,eclipse-che-cheworkspaces-devworkspace-clusterrole,test-roles-1,test-roles-2,test-roles-3",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).Build()
			serverReconciler := NewCheServerReconciler()

			cheEnv, err := serverReconciler.getConfigMapData(ctx)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedUserClusterRoles, cheEnv["CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"])
		})
	}
}
