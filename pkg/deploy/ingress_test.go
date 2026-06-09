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

package deploy

import (
	"context"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"testing"
)

func TestIngressSpec(t *testing.T) {
	type testCase struct {
		name            string
		expectedIngress *networkingv1.Ingress
		cheCluster      *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name: "Test case #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Hostname:         "test-host",
						Labels:           map[string]string{"type": "default"},
						Annotations:      map[string]string{"annotation-key": "annotation-value"},
						IngressClassName: "nginx",
					},
				},
			},
			expectedIngress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  defaults.GetCheFlavor(),
						"app.kubernetes.io/instance":   defaults.GetCheFlavor(),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": defaults.GetCheFlavor() + "-operator",
						"app.kubernetes.io/name":       defaults.GetCheFlavor(),
					},
					Annotations: map[string]string{
						"che.eclipse.org/managed-annotations-digest": "0000",
						"annotation-key": "annotation-value",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: networkingv1.SchemeGroupVersion.String(),
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
					TLS:              []networkingv1.IngressTLS{{Hosts: []string{"test-host"}}},
					Rules: []networkingv1.IngressRule{
						{
							Host: "test-host",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "che-host",
													Port: networkingv1.ServiceBackendPort{
														Number: 8080,
													},
												},
											},
											Path:     "/",
											PathType: (*networkingv1.PathType)(ptr.To(string(networkingv1.PathTypeImplementationSpecific))),
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test case #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Hostname:    "test-host",
						Labels:      map[string]string{"type": "default"},
						Annotations: map[string]string{"annotation-key": "annotation-value", "kubernetes.io/ingress.class": "nginx"},
					},
				},
			},
			expectedIngress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  defaults.GetCheFlavor(),
						"app.kubernetes.io/instance":   defaults.GetCheFlavor(),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": defaults.GetCheFlavor() + "-operator",
						"app.kubernetes.io/name":       defaults.GetCheFlavor(),
					},
					Annotations: map[string]string{
						"che.eclipse.org/managed-annotations-digest": "0000",
						"annotation-key": "annotation-value",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: networkingv1.SchemeGroupVersion.String(),
				},
				Spec: networkingv1.IngressSpec{
					IngressClassName: ptr.To("nginx"),
					TLS:              []networkingv1.IngressTLS{{Hosts: []string{"test-host"}}},
					Rules: []networkingv1.IngressRule{
						{
							Host: "test-host",
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "che-host",
													Port: networkingv1.ServiceBackendPort{
														Number: 8080,
													},
												},
											},
											Path:     "/",
											PathType: (*networkingv1.PathType)(ptr.To(string(networkingv1.PathTypeImplementationSpecific))),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.NewCtxBuilder().WithCheCluster(testCase.cheCluster).Build()
			_, actualIngress := GetIngressSpec(ctx,
				"test",
				"",
				"che-host",
				8080,
				defaults.GetCheFlavor(),
			)
			assert.Equal(t, testCase.expectedIngress, actualIngress)
		})
	}
}

func TestSyncIngressToCluster(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "host-1",
			},
		},
	}

	deployContext := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()
	_, done, err := SyncIngressToCluster(deployContext, "test", "", "service-1", 8080, "component")
	assert.Nil(t, err)
	assert.True(t, done)

	cheCluster.Spec.Networking.Hostname = "host-2"
	_, done, err = SyncIngressToCluster(deployContext, "test", "", "service-2", 8080, "component")
	assert.Nil(t, err)
	assert.True(t, done)

	actual := &networkingv1.Ingress{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	assert.Nil(t, err)

	assert.Equal(t, "host-2", actual.Spec.Rules[0].Host)
	assert.Equal(t, "service-2", actual.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
}
