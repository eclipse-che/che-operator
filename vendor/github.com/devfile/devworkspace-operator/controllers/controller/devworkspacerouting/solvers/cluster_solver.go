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

package solvers

import (
	"fmt"

	"github.com/devfile/devworkspace-operator/pkg/common"
	"github.com/devfile/devworkspace-operator/pkg/constants"

	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
)

const (
	serviceServingCertAnnot = "service.beta.openshift.io/serving-cert-secret-name"
)

type ClusterSolver struct {
	TLS bool
}

var _ RoutingSolver = (*ClusterSolver)(nil)

func (s *ClusterSolver) FinalizerRequired(*controllerv1alpha1.DevWorkspaceRouting) bool {
	return false
}

func (s *ClusterSolver) Finalize(*controllerv1alpha1.DevWorkspaceRouting) error {
	return nil
}

func (s *ClusterSolver) GetSpecObjects(routing *controllerv1alpha1.DevWorkspaceRouting, workspaceMeta DevWorkspaceMetadata) (RoutingObjects, error) {
	spec := routing.Spec
	services := getServicesForEndpoints(spec.Endpoints, workspaceMeta)
	podAdditions := &controllerv1alpha1.PodAdditions{}
	if s.TLS {
		readOnlyMode := int32(420)
		for idx, service := range services {
			if services[idx].Annotations == nil {
				services[idx].Annotations = map[string]string{}
			}
			services[idx].Annotations[serviceServingCertAnnot] = service.Name
			podAdditions.Volumes = append(podAdditions.Volumes, corev1.Volume{
				Name: common.ServingCertVolumeName(service.Name),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  service.Name,
						DefaultMode: &readOnlyMode,
					},
				},
			})
			podAdditions.VolumeMounts = append(podAdditions.VolumeMounts, corev1.VolumeMount{
				Name:      common.ServingCertVolumeName(service.Name),
				ReadOnly:  true,
				MountPath: "/var/serving-cert/",
			})
		}
	}

	return RoutingObjects{
		Services:     services,
		PodAdditions: podAdditions,
	}, nil
}

func (s *ClusterSolver) GetExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {

	exposedEndpoints = map[string]controllerv1alpha1.ExposedEndpointList{}

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Exposure == dw.NoneEndpointExposure {
				continue
			}
			url, err := resolveServiceHostnameForEndpoint(endpoint, routingObj.Services)
			if err != nil {
				return nil, false, err
			}

			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], controllerv1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        url,
				Attributes: endpoint.Attributes,
			})
		}
	}

	return exposedEndpoints, true, nil
}

func resolveServiceHostnameForEndpoint(endpoint dw.Endpoint, services []corev1.Service) (string, error) {
	for _, service := range services {
		if service.Annotations[constants.DevWorkspaceDiscoverableServiceAnnotation] == "true" {
			continue
		}
		for _, servicePort := range service.Spec.Ports {
			if servicePort.Port == int32(endpoint.TargetPort) {
				return getHostnameFromService(service, servicePort.Port), nil
			}
		}
	}
	return "", fmt.Errorf("could not find service for endpoint %s", endpoint.Name)
}

func getHostnameFromService(service corev1.Service, port int32) string {
	scheme := "http"
	if _, ok := service.Annotations[serviceServingCertAnnot]; ok {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s.%s.svc:%d", scheme, service.Name, service.Namespace, port)
}
