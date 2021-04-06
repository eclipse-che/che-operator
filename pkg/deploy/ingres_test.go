//
// Copyright (c) 2021 Red Hat, Inc.
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
	"os"
	"reflect"

	"github.com/google/go-cmp/cmp"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

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
		initObjects           []runtime.Object
		ingressCustomSettings orgv1.IngressCustomSettings
		expectedIngress       *v1beta1.Ingress
	}

	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
		},
	}

	testCases := []testCase{
		{
			name:             "Test custom host",
			ingressName:      "test",
			ingressComponent: "test-component",
			ingressHost:      "test-host",
			ingressPath:      "",
			serviceName:      "che",
			servicePort:      8080,
			ingressCustomSettings: orgv1.IngressCustomSettings{
				Labels: "type=default",
			},
			initObjects: []runtime.Object{},
			expectedIngress: &v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   DefaultCheFlavor(cheCluster),
						"app.kubernetes.io/managed-by": DefaultCheFlavor(cheCluster) + "-operator",
						"app.kubernetes.io/name":       DefaultCheFlavor(cheCluster),
					},
					Annotations: map[string]string{
						"kubernetes.io/ingress.class":                       "nginx",
						"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
						"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
						"nginx.ingress.kubernetes.io/ssl-redirect":          "false",
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Ingress",
					APIVersion: v1beta1.SchemeGroupVersion.String(),
				},
				Spec: v1beta1.IngressSpec{
					Rules: []v1beta1.IngressRule{
						{
							Host: "test-host",
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Backend: v1beta1.IngressBackend{
												ServiceName: "che",
												ServicePort: intstr.FromInt(8080),
											},
											Path: "/",
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
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &DeployContext{
				CheCluster: cheCluster,
				ClusterAPI: ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
			}

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
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	_, done, err := SyncIngressToCluster(deployContext, "test", "host-1", "", "service-1", 8080, orgv1.IngressCustomSettings{}, "component")
	if !done || err != nil {
		t.Fatalf("Failed to sync ingress: %v", err)
	}

	_, done, err = SyncIngressToCluster(deployContext, "test", "host-2", "", "service-2", 8080, orgv1.IngressCustomSettings{}, "component")
	if !done || err != nil {
		t.Fatalf("Failed to sync ingress: %v", err)
	}

	actual := &v1beta1.Ingress{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test"}, actual)
	if err != nil {
		t.Fatalf("Failed to get ingress: %v", err)
	}

	if actual.Spec.Rules[0].Host != "host-2" {
		t.Fatalf("Failed to sync ingress")
	}
	if actual.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName != "service-2" {
		t.Fatalf("Failed to sync ingress")
	}
}
