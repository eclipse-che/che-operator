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

	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"testing"
)

func TestRouteSpec(t *testing.T) {
	weight := int32(100)

	type testCase struct {
		name           string
		routeName      string
		routePath      string
		routeComponent string
		serviceName    string
		servicePort    int32
		cheCluster     *chev2.CheCluster
		expectedRoute  *routev1.Route
	}

	testCases := []testCase{
		{
			name:           "Test domain",
			routeName:      "test",
			routeComponent: "test-component",
			serviceName:    "che",
			servicePort:    8080,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Labels: map[string]string{"type": "default"},
						Domain: "route-domain",
					},
				},
			},
			expectedRoute: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   defaults.GetCheFlavor(),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": defaults.GetCheFlavor() + "-operator",
						"app.kubernetes.io/name":       defaults.GetCheFlavor(),
					},
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "Route",
					APIVersion: routev1.SchemeGroupVersion.String(),
				},
				Spec: routev1.RouteSpec{
					Host: map[bool]string{false: "eclipse-che.route-domain", true: "devspaces.route-domain"}[defaults.GetCheFlavor() == "devspaces"],
					To: routev1.RouteTargetReference{
						Kind:   "Service",
						Name:   "che",
						Weight: &weight,
					},
					TLS: &routev1.TLSConfig{Termination: routev1.TLSTerminationEdge, InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect},
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
			serviceName:    "che",
			servicePort:    8080,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Labels:   map[string]string{"type": "default"},
						Hostname: "test-host",
					},
				},
			},
			expectedRoute: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
					Labels: map[string]string{
						"type":                         "default",
						"app.kubernetes.io/component":  "test-component",
						"app.kubernetes.io/instance":   defaults.GetCheFlavor(),
						"app.kubernetes.io/part-of":    "che.eclipse.org",
						"app.kubernetes.io/managed-by": defaults.GetCheFlavor() + "-operator",
						"app.kubernetes.io/name":       defaults.GetCheFlavor(),
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
					TLS: &routev1.TLSConfig{Termination: routev1.TLSTerminationEdge, InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect},
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
			deployContext := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			actualRoute, err := GetRouteSpec(deployContext,
				testCase.routeName,
				testCase.routePath,
				testCase.serviceName,
				testCase.servicePort,
				testCase.routeComponent,
			)

			assert.Nil(t, err)
			assert.Equal(t, testCase.expectedRoute, actualRoute)
		})
	}
}

func TestSyncRouteToCluster(t *testing.T) {
	deployContext := test.GetDeployContext(nil, []runtime.Object{})

	done, err := SyncRouteToCluster(deployContext, "test", "", "service", 80, "test")
	assert.Nil(t, err)
	assert.True(t, done)

	// sync another route
	done, err = SyncRouteToCluster(deployContext, "test", "", "service", 90, "test")
	assert.Nil(t, err)
	assert.True(t, done)

	actual := &routev1.Route{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	assert.Nil(t, err)
	assert.Equal(t, int32(90), actual.Spec.Port.TargetPort.IntVal)

	// sync route with labels & domain
	deployContext.CheCluster.Spec.Networking.Domain = "domain"
	deployContext.CheCluster.Spec.Networking.Labels = map[string]string{"a": "b"}
	done, err = SyncRouteToCluster(deployContext, "test", "", "service", 90, "test")
	assert.Nil(t, err)
	assert.True(t, done)

	actual = &routev1.Route{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	assert.Nil(t, err)
	assert.Equal(t, "b", actual.ObjectMeta.Labels["a"])

	expectedHost := map[bool]string{false: "eclipse-che.domain", true: "devspaces.domain"}[defaults.GetCheFlavor() == "devspaces"]
	assert.Equal(t, expectedHost, actual.Spec.Host)

	// sync route with annotations
	deployContext.CheCluster.Spec.Networking.Annotations = map[string]string{"a": "b"}
	done, err = SyncRouteToCluster(deployContext, "test", "", "service", 90, "test")

	actual = &routev1.Route{}
	err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	assert.Nil(t, err)
	assert.True(t, done)
	assert.Equal(t, "b", actual.ObjectMeta.Annotations["a"])
	assert.NotEmpty(t, actual.ObjectMeta.Annotations[constants.CheEclipseOrgManagedAnnotationsDigest])
}
