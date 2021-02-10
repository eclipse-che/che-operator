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
	"os"
	"reflect"

	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"testing"
)

func TestRouteSpec(t *testing.T) {
	weight := int32(100)
	_true := true

	type testCase struct {
		name                string
		routeName           string
		routeHost           string
		routeComponent      string
		serviceName         string
		servicePort         int32
		initObjects         []runtime.Object
		routeCustomSettings orgv1.RouteCustomSettings
		expectedRoute       *routev1.Route
	}

	testCases := []testCase{
		{
			name:           "Test domain",
			routeName:      "test",
			routeComponent: "test-component",
			serviceName:    "che",
			servicePort:    8080,
			routeCustomSettings: orgv1.RouteCustomSettings{
				Labels: "type=default",
				Domain: "route-domain",
			},
			initObjects: []runtime.Object{},
			expectedRoute: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   "che",
						"app.kubernetes.io/managed-by": "che-operator",
						"app.kubernetes.io/name":       "che",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "org.eclipse.che/v1",
							Kind:               "CheCluster",
							Controller:         &_true,
							BlockOwnerDeletion: &_true,
						},
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: routev1.SchemeGroupVersion.String(),
				},
				Spec: routev1.RouteSpec{
					Host: "test-eclipse-che.route-domain",
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "che",
						Weight: &weight,
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: int32(8080),
						},
					},
				},
			},
		},
		{
			name:           "Test custom host",
			routeName:      "test",
			routeComponent: "test-component",
			routeHost:      "test-host",
			serviceName:    "che",
			servicePort:    8080,
			routeCustomSettings: orgv1.RouteCustomSettings{
				Labels: "type=default",
			},
			initObjects: []runtime.Object{},
			expectedRoute: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   "che",
						"app.kubernetes.io/managed-by": "che-operator",
						"app.kubernetes.io/name":       "che",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "org.eclipse.che/v1",
							Kind:               "CheCluster",
							Controller:         &_true,
							BlockOwnerDeletion: &_true,
						},
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: routev1.SchemeGroupVersion.String(),
				},
				Spec: routev1.RouteSpec{
					Host: "test-host",
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "che",
						Weight: &weight,
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: int32(8080),
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
				CheCluster: &orgv1.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "eclipse-che",
					},
				},
				ClusterAPI: ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
			}

			actualRoute, err := GetSpecRoute(deployContext,
				testCase.routeName,
				testCase.routeHost,
				testCase.serviceName,
				testCase.servicePort,
				testCase.routeCustomSettings,
				testCase.routeComponent,
			)
			if err != nil {
				t.Fatalf("Error creating route: %v", err)
			}

			if !reflect.DeepEqual(testCase.expectedRoute, actualRoute) {
				t.Errorf("Expected route and route returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedRoute, actualRoute))
			}
		})
	}
}
