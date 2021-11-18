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
	"fmt"
	"reflect"
	"sort"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// host name template: `<route-name>-<route-namespace>.<domain>`
	HostNameTemplate = "%s-%s.%s"
)

var routeDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routev1.Route{}, "TypeMeta", "Status"),
	cmpopts.IgnoreFields(routev1.RouteSpec{}, "Host", "WildcardPolicy"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels) &&
			x.Annotations[CheEclipseOrgManagedAnnotationsDigest] == y.Annotations[CheEclipseOrgManagedAnnotationsDigest]
	}),
}
var routeWithHostDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routev1.Route{}, "TypeMeta", "Status"),
	cmpopts.IgnoreFields(routev1.RouteSpec{}, "WildcardPolicy"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels) &&
			x.Annotations[CheEclipseOrgManagedAnnotationsDigest] == y.Annotations[CheEclipseOrgManagedAnnotationsDigest]
	}),
}

func SyncRouteToCluster(
	deployContext *DeployContext,
	name string,
	host string,
	path string,
	serviceName string,
	servicePort int32,
	routeCustomSettings orgv1.RouteCustomSettings,
	component string) (bool, error) {

	routeSpec, err := GetRouteSpec(deployContext, name, host, path, serviceName, servicePort, routeCustomSettings, component)
	if err != nil {
		return false, err
	}

	if host != "" {
		return Sync(deployContext, routeSpec, routeWithHostDiffOpts)
	}
	return Sync(deployContext, routeSpec, routeDiffOpts)
}

// GetRouteSpec returns default configuration of a route in Che namespace.
func GetRouteSpec(
	deployContext *DeployContext,
	name string,
	host string,
	path string,
	serviceName string,
	servicePort int32,
	routeCustomSettings orgv1.RouteCustomSettings,
	component string) (*routev1.Route, error) {

	cheFlavor := DefaultCheFlavor(deployContext.CheCluster)
	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
	labels := GetLabels(deployContext.CheCluster, component)
	MergeLabels(labels, routeCustomSettings.Labels)

	// add custom annotations
	var annotations map[string]string
	if len(routeCustomSettings.Annotations) > 0 {
		annotations = make(map[string]string)
		for k, v := range routeCustomSettings.Annotations {
			annotations[k] = v
		}
	}

	// add 'che.eclipse.org/managed-annotations-digest' annotation
	// to store and compare annotations managed by operator only
	annotationsKeys := make([]string, 0, len(annotations))
	for k := range annotations {
		annotationsKeys = append(annotationsKeys, k)
	}
	if len(annotationsKeys) > 0 {
		sort.Strings(annotationsKeys)

		data := ""
		for _, k := range annotationsKeys {
			data += k + ":" + annotations[k] + ","
		}
		if util.IsTestMode() {
			annotations[CheEclipseOrgManagedAnnotationsDigest] = "0000"
		} else {
			annotations[CheEclipseOrgManagedAnnotationsDigest] = util.ComputeHash256([]byte(data))
		}
	}

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
			Name:        name,
			Namespace:   deployContext.CheCluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
	}

	route.Spec = routev1.RouteSpec{
		To: routev1.RouteTargetReference{
			Kind:   "Service",
			Name:   serviceName,
			Weight: &weight,
		},
		Path: path,
		Port: &routev1.RoutePort{
			TargetPort: targetPort,
		},
	}

	if host != "" {
		route.Spec.Host = host
	} else if routeCustomSettings.Domain != "" {
		route.Spec.Host = fmt.Sprintf(HostNameTemplate, route.ObjectMeta.Name, route.ObjectMeta.Namespace, routeCustomSettings.Domain)
	}

	if tlsSupport {
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}

		// for server and dashboard ingresses
		if (component == cheFlavor || component == cheFlavor+"-dashboard") && deployContext.CheCluster.Spec.Server.CheHostTLSSecret != "" {
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

	return route, nil
}
