//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"

	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IngressExposer struct {
	devWorkspaceID string
	baseDomain     string
	tlsSecretName  string
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
	annotations   map[string]string
}

// This method is used compose the object names (both Kubernetes objects and "objects" within Traefik configuration)
// representing object endpoints.
func getEndpointExposingObjectName(componentName string, workspaceID string, port int32, endpointName string) string {
	if endpointName == "" {
		return fmt.Sprintf("%s-%s-%d", workspaceID, componentName, port)
	}
	return fmt.Sprintf("%s-%s-%d-%s", workspaceID, componentName, port, endpointName)
}

func (e *RouteExposer) initFrom(ctx context.Context, cl client.Client, cluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	e.baseDomain = cluster.Status.WorkspaceBaseDomain
	e.devWorkspaceID = routing.Spec.DevWorkspaceId

	if cluster.Spec.Networking.TlsSecretName != "" {
		secret := &corev1.Secret{}
		err := cl.Get(ctx, client.ObjectKey{Name: cluster.Spec.Networking.TlsSecretName, Namespace: cluster.Namespace}, secret)
		if err != nil {
			return err
		}

		e.tlsSecretKey = string(secret.Data["tls.key"])
		e.tlsSecretCertificate = string(secret.Data["tls.crt"])
	}

	return nil
}

func (e *IngressExposer) initFrom(ctx context.Context, cl client.Client, cluster *chev2.CheCluster, routing *dwo.DevWorkspaceRouting) error {
	e.baseDomain = cluster.Status.WorkspaceBaseDomain
	e.devWorkspaceID = routing.Spec.DevWorkspaceId

	if cluster.Spec.Networking.TlsSecretName != "" {
		tlsSecretName := routing.Spec.DevWorkspaceId + "-endpoints"
		e.tlsSecretName = tlsSecretName

		secret := &corev1.Secret{}

		// check that there is no secret with the anticipated name yet
		err := cl.Get(ctx, client.ObjectKey{Name: tlsSecretName, Namespace: routing.Namespace}, secret)
		if errors.IsNotFound(err) {
			secret = &corev1.Secret{}
			err = cl.Get(ctx, client.ObjectKey{Name: cluster.Spec.Networking.TlsSecretName, Namespace: cluster.Namespace}, secret)
			if err != nil {
				return err
			}

			yes := true

			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tlsSecretName,
					Namespace: routing.Namespace,
					Labels: map[string]string{
						constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
					},
					OwnerReferences: []metav1.OwnerReference{
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

func (e *RouteExposer) getRouteForService(
	ctx context.Context,
	endpoint *EndpointInfo,
	endpointStrategy EndpointStrategy,
	cl client.Client,
	cheCluster *chev2.CheCluster,
) (*routev1.Route, error) {
	annotations := map[string]string{}
	utils.AddMap(annotations, endpoint.annotations)
	utils.AddMap(annotations, map[string]string{
		defaults.ConfigAnnotationEndpointName:  endpoint.endpointName,
		defaults.ConfigAnnotationComponentName: endpoint.componentName,
	})

	labels := map[string]string{}
	utils.AddMap(labels, map[string]string{
		dwconstants.DevWorkspaceIDLabel:    e.devWorkspaceID,
		constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
	})

	if cheCluster.IsDevEnvironmentExternalTLSConfigEnabled() && isSecureScheme(endpoint.scheme) {
		// set labels and annotations only for secure endpoints
		// otherwise it might trigger external tool to set up TLS for insecure endpoints
		utils.AddMap(labels, cheCluster.Spec.DevEnvironments.Networking.ExternalTLSConfig.Labels)
		utils.AddMap(annotations, cheCluster.Spec.DevEnvironments.Networking.ExternalTLSConfig.Annotations)
	} else {
		// TODO it is needed to apply custom annotations as well
		// https://github.com/eclipse-che/che/issues/23118
		// To be compatible from CheCluster API v1 configuration
		routeLabels := cheCluster.Spec.Components.CheServer.ExtraProperties["CHE_INFRA_OPENSHIFT_ROUTE_LABELS"]
		if routeLabels != "" {
			utils.AddMap(labels, utils.ParseMap(routeLabels))
		}
		utils.AddMap(labels, cheCluster.Spec.Networking.Labels)
	}

	targetEndpoint := intstr.FromInt32(endpoint.port)
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:            getEndpointExposingObjectName(endpoint.componentName, e.devWorkspaceID, endpoint.port, endpoint.endpointName),
			Namespace:       endpoint.service.Namespace,
			Labels:          labels,
			Annotations:     annotations,
			OwnerReferences: endpoint.service.OwnerReferences,
		},
		Spec: routev1.RouteSpec{
			Host: endpointStrategy.getHostname(endpoint, e.baseDomain),
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
		if cheCluster.IsDevEnvironmentExternalTLSConfigEnabled() {
			// fetch existed route from the cluster and copy TLS config
			// in order avoid resyncing by devworkspace controller
			clusterRoute := &routev1.Route{}
			if err := cl.Get(ctx, client.ObjectKey{Name: route.Name, Namespace: route.Namespace}, clusterRoute); err == nil {
				route.Spec.TLS = clusterRoute.Spec.TLS
			} else if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			route.Spec.TLS = &routev1.TLSConfig{
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
				Termination:                   routev1.TLSTerminationEdge,
			}

			if e.tlsSecretKey != "" {
				route.Spec.TLS.Key = e.tlsSecretKey
				route.Spec.TLS.Certificate = e.tlsSecretCertificate
			}
		}
	}

	return route, nil
}

func (e *IngressExposer) getIngressForService(
	ctx context.Context,
	endpoint *EndpointInfo,
	endpointStrategy EndpointStrategy,
	cl client.Client,
	cheCluster *chev2.CheCluster,
) (*networkingv1.Ingress, error) {
	annotations := map[string]string{}
	utils.AddMap(annotations, endpoint.annotations)
	utils.AddMap(annotations, map[string]string{
		defaults.ConfigAnnotationEndpointName:  endpoint.endpointName,
		defaults.ConfigAnnotationComponentName: endpoint.componentName,
	})

	labels := map[string]string{}
	utils.AddMap(labels, map[string]string{
		dwconstants.DevWorkspaceIDLabel:    e.devWorkspaceID,
		constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
	})

	if cheCluster.IsDevEnvironmentExternalTLSConfigEnabled() && isSecureScheme(endpoint.scheme) {
		// set labels and annotations only for secure endpoints
		// otherwise it might trigger external tool to set up TLS for insecure endpoints
		utils.AddMap(labels, cheCluster.Spec.DevEnvironments.Networking.ExternalTLSConfig.Labels)
		utils.AddMap(annotations, cheCluster.Spec.DevEnvironments.Networking.ExternalTLSConfig.Annotations)
	} else {
		// TODO it is needed to apply custom labels as well
		// https://github.com/eclipse-che/che/issues/23118
		if len(cheCluster.Spec.Networking.Annotations) > 0 {
			utils.AddMap(annotations, cheCluster.Spec.Networking.Annotations)
		} else {
			utils.AddMap(annotations, deploy.DefaultIngressAnnotations)
		}
	}

	hostname := endpointStrategy.getHostname(endpoint, e.baseDomain)
	ingressPathType := networkingv1.PathTypeImplementationSpecific

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            getEndpointExposingObjectName(endpoint.componentName, e.devWorkspaceID, endpoint.port, endpoint.endpointName),
			Namespace:       endpoint.service.Namespace,
			Labels:          labels,
			Annotations:     annotations,
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

	if isSecureScheme(endpoint.scheme) {
		if cheCluster.IsDevEnvironmentExternalTLSConfigEnabled() {
			// fetch existed ingress from the cluster and copy TLS config
			// in order avoid resyncing by devworkspace controller
			clusterIngress := &networkingv1.Ingress{}
			if err := cl.Get(ctx, client.ObjectKey{Name: ingress.Name, Namespace: ingress.Namespace}, clusterIngress); err == nil {
				ingress.Spec.TLS = clusterIngress.Spec.TLS
			} else if !errors.IsNotFound(err) {
				return nil, err
			}
		} else if e.tlsSecretName != "" {
			ingress.Spec.TLS = []networkingv1.IngressTLS{
				{
					Hosts:      []string{hostname},
					SecretName: e.tlsSecretName,
				},
			}
		}
	}

	return ingress, nil
}
