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

package devworkspace

import (
	"context"
	"testing"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileDevWorkspaceConfigTLSCertificateConfigmapRef(t *testing.T) {
	type testCase struct {
		name                  string
		cheCluster            *chev2.CheCluster
		existedObjects        []client.Object
		expectedRoutingConfig *controllerv1alpha1.RoutingConfig
	}

	var testCases = []testCase{
		{
			name: "Create DevWorkspaceOperatorConfig with TLSCertificateConfigmapRef when CA bundle ConfigMap exists with data",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{
						"ca-bundle.crt": "certificate-data",
					},
				},
			},
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
					Name:      tls.CheMergedCABundleCertsCMName,
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name: "Update existing DevWorkspaceOperatorConfig with TLSCertificateConfigmapRef when CA bundle ConfigMap exists with data",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{
						"ca-bundle.crt": "certificate-data",
					},
				},
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Routing: &controllerv1alpha1.RoutingConfig{
							DefaultRoutingClass: "che",
						},
					},
				},
			},
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				DefaultRoutingClass: "che",
				TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
					Name:      tls.CheMergedCABundleCertsCMName,
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name: "Do not set TLSCertificateConfigmapRef when CA bundle ConfigMap does not exist",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			expectedRoutingConfig: nil,
		},
		{
			name: "Do not set TLSCertificateConfigmapRef when CA bundle ConfigMap exists but has no data",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{},
				},
			},
			expectedRoutingConfig: nil,
		},
		{
			name: "Clear TLSCertificateConfigmapRef when CA bundle ConfigMap is removed",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Routing: &controllerv1alpha1.RoutingConfig{
							TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
								Name:      tls.CheMergedCABundleCertsCMName,
								Namespace: "eclipse-che",
							},
						},
					},
				},
			},
			expectedRoutingConfig: nil,
		},
		{
			name: "Clear TLSCertificateConfigmapRef when CA bundle ConfigMap data is emptied",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{},
				},
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Routing: &controllerv1alpha1.RoutingConfig{
							TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
								Name:      tls.CheMergedCABundleCertsCMName,
								Namespace: "eclipse-che",
							},
						},
					},
				},
			},
			expectedRoutingConfig: nil,
		},
		{
			name: "Re-reconcile is stable when TLSCertificateConfigmapRef already matches",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{
						"ca-bundle.crt": "certificate-data",
					},
				},
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Routing: &controllerv1alpha1.RoutingConfig{
							TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
								Name:      tls.CheMergedCABundleCertsCMName,
								Namespace: "eclipse-che",
							},
						},
					},
				},
			},
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
					Name:      tls.CheMergedCABundleCertsCMName,
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name: "Clear TLSCertificateConfigmapRef while preserving other routing config",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Routing: &controllerv1alpha1.RoutingConfig{
							DefaultRoutingClass: "che",
							TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
								Name:      tls.CheMergedCABundleCertsCMName,
								Namespace: "eclipse-che",
							},
						},
					},
				},
			},
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				DefaultRoutingClass: "che",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      devWorkspaceConfigName,
					Namespace: testCase.cheCluster.Namespace,
				},
				dwoc,
			)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedRoutingConfig, dwoc.Config.Routing)
		})
	}
}

func TestReconcileDevWorkspaceConfigProxyAndTLSComposition(t *testing.T) {
	type testCase struct {
		name                  string
		cheCluster            *chev2.CheCluster
		existedObjects        []client.Object
		httpProxy             string
		httpsProxy            string
		expectedRoutingConfig *controllerv1alpha1.RoutingConfig
	}

	var testCases = []testCase{
		{
			name: "Both proxy and TLS certificate configmap ref are set on RoutingConfig",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{
						"ca-bundle.crt": "certificate-data",
					},
				},
			},
			httpProxy:  "http://proxy.example.com:3128",
			httpsProxy: "https://proxy.example.com:3128",
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
					Name:      tls.CheMergedCABundleCertsCMName,
					Namespace: "eclipse-che",
				},
				ProxyConfig: &controllerv1alpha1.Proxy{
					HttpProxy:  ptr.To(""),
					HttpsProxy: ptr.To(""),
					NoProxy:    ptr.To(""),
				},
			},
		},
		{
			name: "Proxy set without TLS certificate configmap does not clobber routing",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			httpProxy:  "http://proxy.example.com:3128",
			httpsProxy: "https://proxy.example.com:3128",
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				ProxyConfig: &controllerv1alpha1.Proxy{
					HttpProxy:  ptr.To(""),
					HttpsProxy: ptr.To(""),
					NoProxy:    ptr.To(""),
				},
			},
		},
		{
			name: "TLS certificate configmap ref set without proxy does not clobber routing",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{
						"ca-bundle.crt": "certificate-data",
					},
				},
			},
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
					Name:      tls.CheMergedCABundleCertsCMName,
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name: "Both proxy and TLS compose with existing routing config",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
			existedObjects: []client.Object{
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tls.CheMergedCABundleCertsCMName,
						Namespace: "eclipse-che",
					},
					Data: map[string]string{
						"ca-bundle.crt": "certificate-data",
					},
				},
				&controllerv1alpha1.DevWorkspaceOperatorConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      devWorkspaceConfigName,
						Namespace: "eclipse-che",
					},
					TypeMeta: metav1.TypeMeta{
						Kind:       "DevWorkspaceOperatorConfig",
						APIVersion: controllerv1alpha1.GroupVersion.String(),
					},
					Config: &controllerv1alpha1.OperatorConfiguration{
						Routing: &controllerv1alpha1.RoutingConfig{
							DefaultRoutingClass: "che",
						},
					},
				},
			},
			httpProxy:  "http://proxy.example.com:3128",
			httpsProxy: "https://proxy.example.com:3128",
			expectedRoutingConfig: &controllerv1alpha1.RoutingConfig{
				DefaultRoutingClass: "che",
				TLSCertificateConfigmapRef: &controllerv1alpha1.ConfigmapReference{
					Name:      tls.CheMergedCABundleCertsCMName,
					Namespace: "eclipse-che",
				},
				ProxyConfig: &controllerv1alpha1.Proxy{
					HttpProxy:  ptr.To(""),
					HttpsProxy: ptr.To(""),
					NoProxy:    ptr.To(""),
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.existedObjects...).Build()
			deployContext.Proxy.HttpProxy = testCase.httpProxy
			deployContext.Proxy.HttpsProxy = testCase.httpsProxy

			devWorkspaceConfigReconciler := NewDevWorkspaceConfigReconciler()
			test.EnsureReconcile(t, deployContext, devWorkspaceConfigReconciler.Reconcile)

			dwoc := &controllerv1alpha1.DevWorkspaceOperatorConfig{}
			err := deployContext.ClusterAPI.Client.Get(
				context.TODO(),
				types.NamespacedName{
					Name:      devWorkspaceConfigName,
					Namespace: testCase.cheCluster.Namespace,
				},
				dwoc,
			)

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedRoutingConfig, dwoc.Config.Routing)
		})
	}
}
