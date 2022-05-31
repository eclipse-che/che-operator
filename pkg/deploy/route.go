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
	"strings"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var routeDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(routev1.Route{}, "TypeMeta", "Status"),
	cmpopts.IgnoreFields(routev1.RouteSpec{}, "WildcardPolicy"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels) &&
			x.Annotations[constants.CheEclipseOrgManagedAnnotationsDigest] == y.Annotations[constants.CheEclipseOrgManagedAnnotationsDigest]
	}),
}

func SyncRouteToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	path string,
	serviceName string,
	servicePort int32,
	component string) (bool, error) {

	routeSpec, err := GetRouteSpec(deployContext, name, path, serviceName, servicePort, component)
	if err != nil {
		return false, err
	}

	return Sync(deployContext, routeSpec, routeDiffOpts)
}

// GetRouteSpec returns default configuration of a route in Che namespace.
func GetRouteSpec(
	deployContext *chetypes.DeployContext,
	name string,
	path string,
	serviceName string,
	servicePort int32,
	component string) (*routev1.Route, error) {

	labels := GetLabels(component)
	for k, v := range deployContext.CheCluster.Spec.Networking.Labels {
		labels[k] = v
	}

	// add custom annotations
	var annotations map[string]string
	if len(deployContext.CheCluster.Spec.Networking.Annotations) > 0 {
		annotations = make(map[string]string)
		for k, v := range deployContext.CheCluster.Spec.Networking.Annotations {
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
		if test.IsTestMode() {
			annotations[constants.CheEclipseOrgManagedAnnotationsDigest] = "0000"
		} else {
			annotations[constants.CheEclipseOrgManagedAnnotationsDigest] = utils.ComputeHash256([]byte(data))
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

	route.Spec.Host = deployContext.CheCluster.Spec.Networking.Hostname
	if route.Spec.Host == "" {
		hostSuffix := deployContext.CheCluster.Spec.Networking.Domain

		if hostSuffix == "" {
			existedRoute := &routev1.Route{}
			exists, _ := GetNamespacedObject(deployContext, name, existedRoute)
			if exists {
				// Get route domain from host
				domainEntries := strings.SplitN(existedRoute.Spec.Host, ".", 2)
				if len(domainEntries) == 2 {
					hostSuffix = domainEntries[1]
				}
			}
		}

		// Usually host has the following format: <name>-<namespace>.<domain>
		// If we know domain then we can create a route with a shorter host: <namespace>.<domain>
		if hostSuffix != "" {
			hostPrefix := "eclipse-che"
			if defaults.GetCheFlavor() == "devspaces" {
				hostPrefix = "devspaces"
			}

			route.Spec.Host = fmt.Sprintf("%s.%s", hostPrefix, hostSuffix)
		}
	}

	route.Spec.TLS = &routev1.TLSConfig{
		InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		Termination:                   routev1.TLSTerminationEdge,
	}

	// for server and dashboard ingresses
	if deployContext.CheCluster.Spec.Networking.TlsSecretName != "" {
		secret := &corev1.Secret{}
		namespacedName := types.NamespacedName{
			Namespace: deployContext.CheCluster.Namespace,
			Name:      deployContext.CheCluster.Spec.Networking.TlsSecretName,
		}
		if err := deployContext.ClusterAPI.Client.Get(context.TODO(), namespacedName, secret); err != nil {
			return nil, err
		}

		route.Spec.TLS.Key = string(secret.Data["tls.key"])
		route.Spec.TLS.Certificate = string(secret.Data["tls.crt"])
	}

	return route, nil
}
