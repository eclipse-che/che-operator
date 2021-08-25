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
package solver

import (
	"context"
	"fmt"

	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IngressExposer struct {
	devWorkspaceID     string
	baseDomain         string
	ingressAnnotations map[string]string
	tlsSecretName      string
}

type RouteExposer struct {
	devWorkspaceID       string
	baseDomain           string
	tlsSecretKey         string
	tlsSecretCertificate string
}

type EndpointInfo struct {
	order         int
	componentName string
	endpointName  string
	port          int32
	scheme        string
	service       *corev1.Service
}

// This method is used compose the object names (both Kubernetes objects and "objects" within Traefik configuration)
// representing object endpoints.
func getEndpointExposingObjectName(componentName string, workspaceID string, port int32, endpointName string) string {
	if endpointName == "" {
		return fmt.Sprintf("%s-%s-%d", workspaceID, componentName, port)
	}
	return fmt.Sprintf("%s-%s-%d-%s", workspaceID, componentName, port, endpointName)
}

func (e *RouteExposer) initFrom(ctx context.Context, cl client.Client, cluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	e.baseDomain = cluster.Status.WorkspaceBaseDomain
	e.devWorkspaceID = routing.Spec.DevWorkspaceId

	if cluster.Spec.WorkspaceDomainEndpoints.TlsSecretName != "" {
		secret := &corev1.Secret{}
		err := cl.Get(ctx, client.ObjectKey{Name: cluster.Spec.TlsSecretName, Namespace: cluster.Namespace}, secret)
		if err != nil {
			return err
		}

		e.tlsSecretKey = string(secret.Data["tls.key"])
		e.tlsSecretCertificate = string(secret.Data["tls.crt"])
	}

	return nil
}

func (e *IngressExposer) initFrom(ctx context.Context, cl client.Client, cluster *v2alpha1.CheCluster, routing *dwo.DevWorkspaceRouting, ingressAnnotations map[string]string) error {
	e.baseDomain = cluster.Status.WorkspaceBaseDomain
	e.devWorkspaceID = routing.Spec.DevWorkspaceId
	e.ingressAnnotations = ingressAnnotations

	if cluster.Spec.WorkspaceDomainEndpoints.TlsSecretName != "" {
		tlsSecretName := routing.Spec.DevWorkspaceId + "-endpoints"
		e.tlsSecretName = tlsSecretName

		secret := &corev1.Secret{}

		// check that there is no secret with the anticipated name yet
		err := cl.Get(ctx, client.ObjectKey{Name: tlsSecretName, Namespace: routing.Namespace}, secret)
		if errors.IsNotFound(err) {
			secret = &corev1.Secret{}
			err = cl.Get(ctx, client.ObjectKey{Name: cluster.Spec.TlsSecretName, Namespace: cluster.Namespace}, secret)
			if err != nil {
				return err
			}

			yes := true

			newSecret := &corev1.Secret{
				ObjectMeta: v1.ObjectMeta{
					Name:      tlsSecretName,
					Namespace: routing.Namespace,
					OwnerReferences: []v1.OwnerReference{
						{
							Name:               routing.Name,
							Kind:               routing.Kind,
							APIVersion:         routing.APIVersion,
							UID:                routing.UID,
							Controller:         &yes,
							BlockOwnerDeletion: &yes,
						},
					},
				},
				Type: secret.Type,
				Data: secret.Data,
			}

			return cl.Create(ctx, newSecret)
		}
	}

	return nil
}

func (e *RouteExposer) getRouteForService(endpoint *EndpointInfo) routev1.Route {
	targetEndpoint := intstr.FromInt(int(endpoint.port))
	route := routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getEndpointExposingObjectName(endpoint.componentName, e.devWorkspaceID, endpoint.port, endpoint.endpointName),
			Namespace: endpoint.service.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: e.devWorkspaceID,
			},
			Annotations:     routeAnnotations(endpoint.componentName, endpoint.endpointName),
			OwnerReferences: endpoint.service.OwnerReferences,
		},
		Spec: routev1.RouteSpec{
			Host: hostName(endpoint.order, e.devWorkspaceID, e.baseDomain),
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: endpoint.service.Name,
			},
			Port: &routev1.RoutePort{
				TargetPort: targetEndpoint,
			},
		},
	}

	if isSecureScheme(endpoint.scheme) {
		route.Spec.TLS = &routev1.TLSConfig{
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			Termination:                   routev1.TLSTerminationEdge,
		}

		if e.tlsSecretKey != "" {
			route.Spec.TLS.Key = e.tlsSecretKey
			route.Spec.TLS.Certificate = e.tlsSecretCertificate
		}
	}

	return route
}

func (e *IngressExposer) getIngressForService(endpoint *EndpointInfo) networkingv1.Ingress {
	hostname := hostName(endpoint.order, e.devWorkspaceID, e.baseDomain)
	ingressPathType := networkingv1.PathTypeImplementationSpecific

	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getEndpointExposingObjectName(endpoint.componentName, e.devWorkspaceID, endpoint.port, endpoint.endpointName),
			Namespace: endpoint.service.Namespace,
			Labels: map[string]string{
				constants.DevWorkspaceIDLabel: e.devWorkspaceID,
			},
			Annotations:     finalizeIngressAnnotations(e.ingressAnnotations, endpoint.componentName, endpoint.endpointName),
			OwnerReferences: endpoint.service.OwnerReferences,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: hostname,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: endpoint.service.Name,
											Port: networkingv1.ServiceBackendPort{
												Number: endpoint.port,
											},
										},
									},
									PathType: &ingressPathType,
									Path:     "/",
								},
							},
						},
					},
				},
			},
		},
	}

	if isSecureScheme(endpoint.scheme) && e.tlsSecretName != "" {
		ingress.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{hostname},
				SecretName: e.tlsSecretName,
			},
		}
	}

	return ingress
}

func hostName(order int, workspaceID string, baseDomain string) string {
	return fmt.Sprintf("%s-%d.%s", workspaceID, order+1, baseDomain)
}

func routeAnnotations(machineName string, endpointName string) map[string]string {
	return map[string]string{
		defaults.ConfigAnnotationEndpointName:  endpointName,
		defaults.ConfigAnnotationComponentName: machineName,
	}
}

func finalizeIngressAnnotations(ingressAnnotations map[string]string, machineName string, endpointName string) map[string]string {
	annos := map[string]string{}
	for k, v := range ingressAnnotations {
		annos[k] = v
	}
	annos[defaults.ConfigAnnotationEndpointName] = endpointName
	annos[defaults.ConfigAnnotationComponentName] = machineName

	return annos
}
