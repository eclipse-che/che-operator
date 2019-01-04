//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package operator

import (
	route "github.com/openshift/api/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newRoute(name string, serviceName string) *route.Route {
	var labels = cheLabels
	if name == "keycloak" {
		labels = keycloakLabels
	}
	return &route.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: route.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,

		},
		Spec: route.RouteSpec{
			To: route.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
		},
	}
}

func newTlsRoute(name string, serviceName string) *route.Route {
	var labels = cheLabels
	if name == "keycloak" {
		labels = keycloakLabels
	}
	return &route.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: route.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,

		},
		Spec: route.RouteSpec{
			To: route.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
			TLS: &route.TLSConfig{
				InsecureEdgeTerminationPolicy: route.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   route.TLSTerminationEdge,
			},
		},
	}
}

func createRoute(name string, toService string) (*route.Route, error) {
	rt := newRoute(name, toService)
	if tlsSupport {
		rt = newTlsRoute(name, toService)
	}
	if err := sdk.Create(rt); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+name+" route : %v", err)
		return nil, err
	}
	return rt, nil
}

func CreateRouteIfNotExists(name string, toService string) (*route.Route, error) {
	rt := newRoute(name, toService)
	if tlsSupport {
		rt = newTlsRoute(name, toService)
	}
	err := sdk.Get(rt)
	if err != nil {
		return createRoute(name, toService)
	}
	return rt, nil

}
