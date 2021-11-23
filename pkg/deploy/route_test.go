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
	routev1 "github.com/openshift/api/route/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"testing"
)

func TestRouteSpec(t *testing.T) {
	weight := int32(100)

	type testCase struct {
		name                string
		routeName           string
		routeHost           string
		routePath           string
		routeComponent      string
		serviceName         string
		servicePort         int32
		routeCustomSettings orgv1.RouteCustomSettings
		expectedRoute       *routev1.Route
	}

	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
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
			expectedRoute: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   DefaultCheFlavor(cheCluster),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": DefaultCheFlavor(cheCluster) + "-operator",
						"app.kubernetes.io/name":       DefaultCheFlavor(cheCluster),
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
			expectedRoute: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   DefaultCheFlavor(cheCluster),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": DefaultCheFlavor(cheCluster) + "-operator",
						"app.kubernetes.io/name":       DefaultCheFlavor(cheCluster),
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
			deployContext := GetTestDeployContext(cheCluster, []runtime.Object{})

			actualRoute, err := GetRouteSpec(deployContext,
				testCase.routeName,
				testCase.routeHost,
				testCase.routePath,
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

func TestSyncRouteToCluster(t *testing.T) {
	// init context
	deployContext := GetTestDeployContext(nil, []runtime.Object{})
	routev1.AddToScheme(deployContext.ClusterAPI.Scheme)

	done, err := SyncRouteToCluster(deployContext, "test", "", "", "service", 80, orgv1.RouteCustomSettings{}, "test")
	if !done || err != nil {
		t.Fatalf("Failed to sync route: %v", err)
	}

	// sync another route
	done, err = SyncRouteToCluster(deployContext, "test", "", "", "service", 90, orgv1.RouteCustomSettings{}, "test")
	if !done || err != nil {
		t.Fatalf("Failed to sync route: %v", err)
	}

	actual := &routev1.Route{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get route: %v", err)
	}
	if actual.Spec.Port.TargetPort.IntVal != 90 {
		t.Fatalf("Failed to sync route: %v", err)
	}

	// sync route with labels & domain
	done, err = SyncRouteToCluster(deployContext, "test", "", "", "service", 90, orgv1.RouteCustomSettings{Labels: "a=b", Domain: "domain"}, "test")
	if !done || err != nil {
		t.Fatalf("Failed to sync route: %v", err)
	}

	actual = &routev1.Route{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get route: %v", err)
	}
	if actual.ObjectMeta.Labels["a"] != "b" {
		t.Fatalf("Failed to sync route")
	}
	if actual.Spec.Host != "test-eclipse-che.domain" {
		t.Fatalf("Failed to sync route")
	}

	// sync route with annotations
	done, err = SyncRouteToCluster(deployContext, "test", "", "", "service", 90, orgv1.RouteCustomSettings{Annotations: map[string]string{"a": "b"}}, "test")

	actual = &routev1.Route{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if !done || err != nil {
		t.Fatalf("Failed to sync route: %v", err)
	}
	if actual.ObjectMeta.Annotations["a"] != "b" || actual.ObjectMeta.Annotations[CheEclipseOrgManagedAnnotationsDigest] == "" {
		t.Fatalf("Failed to sync route")
	}
}
