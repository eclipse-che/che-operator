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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type RouteProvisioningStatus struct {
	ProvisioningStatus
	Route *routev1.Route
}

const (
	CheRouteName = "che-host"
)

var routeDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routev1.Route{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(routev1.RouteSpec{}, "Host", "WildcardPolicy"),
}

func SyncRouteToCluster(
	checluster *orgv1.CheCluster,
	name string,
	serviceName string,
	port int32,
	clusterAPI ClusterAPI) RouteProvisioningStatus {

	specRoute, err := GetSpecRoute(checluster, name, serviceName, port, clusterAPI)
	if err != nil {
		return RouteProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	clusterRoute, err := GetClusterRoute(specRoute.Name, specRoute.Namespace, clusterAPI.Client)
	if err != nil {
		return RouteProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterRoute == nil {
		logrus.Infof("Creating a new object: %s, name %s", specRoute.Kind, specRoute.Name)
		err := clusterAPI.Client.Create(context.TODO(), specRoute)
		return RouteProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	diff := cmp.Diff(clusterRoute, specRoute, routeDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterRoute.Kind, clusterRoute.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := clusterAPI.Client.Delete(context.TODO(), clusterRoute)
		if err != nil {
			return RouteProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
			}
		}

		err = clusterAPI.Client.Create(context.TODO(), specRoute)
		return RouteProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	return RouteProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{Continue: true},
		Route:              clusterRoute,
	}
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
func GetSpecRoute(checluster *orgv1.CheCluster, name string, serviceName string, port int32, clusterAPI ClusterAPI) (*routev1.Route, error) {
	tlsSupport := checluster.Spec.Server.TlsSupport
	labels := GetLabels(checluster, DefaultCheFlavor(checluster))
	weight := int32(100)

	if name == "keycloak" {
		labels = GetLabels(checluster, name)
	}
	targetPort := intstr.IntOrString{
		Type:   intstr.Int,
		IntVal: int32(port),
	}
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Route",
			APIVersion: routev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
	}

	route.Spec = routev1.RouteSpec{
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
	}

	err := controllerutil.SetControllerReference(checluster, route, clusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return route, nil
}
