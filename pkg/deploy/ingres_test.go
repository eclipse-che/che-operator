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
	"reflect"

	"github.com/google/go-cmp/cmp"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"testing"
)

func TestIngressSpec(t *testing.T) {
	type testCase struct {
		name                  string
		ingressName           string
		ingressHost           string
		ingressPath           string
		ingressComponent      string
		serviceName           string
		servicePort           int
		ingressCustomSettings orgv1.IngressCustomSettings
		expectedIngress       *networking.Ingress
	}
	cheFlavor := getDefaultFromEnv("CHE_FLAVOR")
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      cheFlavor,
		},
	}

	testCases := []testCase{
		{
			name:             "Test custom host",
			ingressName:      "test",
			ingressComponent: DefaultCheFlavor(cheCluster),
			ingressHost:      "test-host",
			ingressPath:      "",
			serviceName:      "che-host",
			servicePort:      8080,
			ingressCustomSettings: orgv1.IngressCustomSettings{
				Labels:      "type=default",
				Annotations: map[string]string{"annotation-key": "annotation-value"},
			},
			expectedIngress: &networking.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  DefaultCheFlavor(cheCluster),
						"app.kubernetes.io/instance":   DefaultCheFlavor(cheCluster),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": DefaultCheFlavor(cheCluster) + "-operator",
						"app.kubernetes.io/name":       DefaultCheFlavor(cheCluster),
					},
					Annotations: map[string]string{
						"che.eclipse.org/managed-annotations-digest":        "0000",
						"kubernetes.io/ingress.class":                       "nginx",
						"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
						"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
						"nginx.ingress.kubernetes.io/ssl-redirect":          "false",
						"nginx.org/websocket-services":                      "che-host",
						"annotation-key":                                    "annotation-value",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: networking.SchemeGroupVersion.String(),
				},
				Spec: networking.IngressSpec{
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
			deployContext := GetTestDeployContext(cheCluster, []runtime.Object{})

			_, actualIngress := GetIngressSpec(deployContext,
				testCase.ingressName,
				testCase.ingressHost,
				testCase.ingressPath,
				testCase.serviceName,
				testCase.servicePort,
				testCase.ingressCustomSettings,
				testCase.ingressComponent,
			)

			if !reflect.DeepEqual(testCase.expectedIngress, actualIngress) {
				t.Errorf("Expected ingress and ingress returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedIngress, actualIngress))
			}
		})
	}
}

func TestSyncIngressToCluster(t *testing.T) {
	deployContext := GetTestDeployContext(nil, []runtime.Object{})

	_, done, err := SyncIngressToCluster(deployContext, "test", "host-1", "", "service-1", 8080, orgv1.IngressCustomSettings{}, "component")
	if !done || err != nil {
		t.Fatalf("Failed to sync ingress: %v", err)
	}

	_, done, err = SyncIngressToCluster(deployContext, "test", "host-2", "", "service-2", 8080, orgv1.IngressCustomSettings{}, "component")
	if !done || err != nil {
		t.Fatalf("Failed to sync ingress: %v", err)
	}

	actual := &networking.Ingress{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get ingress: %v", err)
	}

	if actual.Spec.Rules[0].Host != "host-2" {
		t.Fatalf("Failed to sync ingress")
	}
	if actual.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name != "service-2" {
		t.Fatalf("Failed to sync ingress")
	}
}
