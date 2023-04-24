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
	"context"

	"github.com/stretchr/testify/assert"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	networking "k8s.io/api/networking/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"testing"
)

func TestIngressSpec(t *testing.T) {
	type testCase struct {
		name            string
		expectedIngress *networking.Ingress
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
			expectedIngress: &networking.Ingress{
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
					APIVersion: networking.SchemeGroupVersion.String(),
				},
				Spec: networking.IngressSpec{
					IngressClassName: pointer.String("nginx"),
					TLS:              []v1.IngressTLS{{Hosts: []string{"test-host"}}},
					Rules: []networking.IngressRule{
						{
							Host: "test-host",
							IngressRuleValue: networking.IngressRuleValue{
								HTTP: &networking.HTTPIngressRuleValue{
									Paths: []networking.HTTPIngressPath{
										{
											Backend: networking.IngressBackend{
												Service: &networking.IngressServiceBackend{
													Name: "che-host",
													Port: networking.ServiceBackendPort{
														Number: 8080,
													},
												},
											},
											Path:     "/",
											PathType: (*networking.PathType)(pointer.StringPtr(string(networking.PathTypeImplementationSpecific))),
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
			expectedIngress: &networking.Ingress{
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
					APIVersion: networking.SchemeGroupVersion.String(),
				},
				Spec: networking.IngressSpec{
					IngressClassName: pointer.String("nginx"),
					TLS:              []v1.IngressTLS{{Hosts: []string{"test-host"}}},
					Rules: []networking.IngressRule{
						{
							Host: "test-host",
							IngressRuleValue: networking.IngressRuleValue{
								HTTP: &networking.HTTPIngressRuleValue{
									Paths: []networking.HTTPIngressPath{
										{
											Backend: networking.IngressBackend{
												Service: &networking.IngressServiceBackend{
													Name: "che-host",
													Port: networking.ServiceBackendPort{
														Number: 8080,
													},
												},
											},
											Path:     "/",
											PathType: (*networking.PathType)(pointer.StringPtr(string(networking.PathTypeImplementationSpecific))),
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
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			_, actualIngress := GetIngressSpec(deployContext,
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

	deployContext := test.GetDeployContext(cheCluster, []runtime.Object{})
	_, done, err := SyncIngressToCluster(deployContext, "test", "", "service-1", 8080, "component")
	assert.Nil(t, err)
	assert.True(t, done)

	cheCluster.Spec.Networking.Hostname = "host-2"
	_, done, err = SyncIngressToCluster(deployContext, "test", "", "service-2", 8080, "component")
	assert.Nil(t, err)
	assert.True(t, done)

	actual := &networking.Ingress{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	assert.Nil(t, err)

	assert.Equal(t, "host-2", actual.Spec.Rules[0].Host)
	assert.Equal(t, "service-2", actual.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name)
}
