//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var routeDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routev1.Route{}, "TypeMeta", "Status"),
	cmpopts.IgnoreFields(routev1.RouteSpec{}, "Host", "WildcardPolicy"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels)
	}),
}
var routeWithHostDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routev1.Route{}, "TypeMeta", "Status"),
	cmpopts.IgnoreFields(routev1.RouteSpec{}, "WildcardPolicy"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels)
	}),
}

func SyncRouteToCluster(
	deployContext *DeployContext,
	name string,
	host string,
	serviceName string,
	servicePort int32,
	additionalLabels string) (*routev1.Route, error) {

	specRoute, err := GetSpecRoute(deployContext, name, host, serviceName, servicePort, additionalLabels)
	if err != nil {
		return nil, err
	}

	clusterRoute, err := GetClusterRoute(specRoute.Name, specRoute.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterRoute == nil {
		logrus.Infof("Creating a new object: %s, name %s", specRoute.Kind, specRoute.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specRoute)
		if !errors.IsAlreadyExists(err) {
			return nil, err
		}
		return nil, nil
	}

	diffOpts := routeDiffOpts
	if host != "" {
		diffOpts = routeWithHostDiffOpts
	}
	diff := cmp.Diff(clusterRoute, specRoute, diffOpts)
	if len(diff) > 0 {
		logrus.Infof("Deleting existed object: %s, name: %s", clusterRoute.Kind, clusterRoute.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := deployContext.ClusterAPI.Client.Delete(context.TODO(), clusterRoute)
		if !errors.IsNotFound(err) {
			return nil, err
		}

		return nil, nil
	}

	return clusterRoute, nil
}

func DeleteRouteIfExists(name string, deployContext *DeployContext) error {
	ingress, err := GetClusterRoute(name, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return err
	}

	if ingress != nil {
		err = deployContext.ClusterAPI.Client.Delete(context.TODO(), ingress)
		if !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// GetClusterRoute returns existing route.
func GetClusterRoute(name string, namespace string, client runtimeClient.Client) (*routev1.Route, error) {
	route := &routev1.Route{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, route)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return route, nil
}

// GetSpecRoute returns default configuration of a route in Che namespace.
func GetSpecRoute(
	deployContext *DeployContext,
	name string,
	host string,
	serviceName string,
	servicePort int32,
	additionalLabels string) (*routev1.Route, error) {

	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
	labels := GetLabels(deployContext.CheCluster, name)
	MergeLabels(labels, additionalLabels)

	weight := int32(100)

	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(servicePort),
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
	}

	route.Spec = routev1.RouteSpec{
		Host: host,
		To: routev1.RouteTargetReference{
			Kind:   "Service",
			Name:   serviceName,
			Weight: &weight,
		},
		Port: &routev1.RoutePort{
			TargetPort: targetPort,
		},
	}

	if tlsSupport {
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}

		if name == DefaultCheFlavor(deployContext.CheCluster) && deployContext.CheCluster.Spec.Server.CheHostTLSSecret != "" {
			secret := &corev1.Secret{}
			namespacedName := types.NamespacedName{
				Namespace: deployContext.CheCluster.Namespace,
				Name:      deployContext.CheCluster.Spec.Server.CheHostTLSSecret,
			}
			if err := deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, secret); err != nil {
				return nil, err
			}

			route.Spec.TLS.Key = string(secret.Data["tls.key"])
			route.Spec.TLS.Certificate = string(secret.Data["tls.crt"])
		}
	}

	err := controllerutil.SetControllerReference(deployContext.CheCluster, route, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return route, nil
}
