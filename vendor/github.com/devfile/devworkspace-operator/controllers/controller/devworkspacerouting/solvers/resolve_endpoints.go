//
// Copyright (c) 2019-2022 Red Hat, Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package solvers

import (
	"fmt"
	"net/url"
	"strings"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/constants"
)

func getExposedEndpoints(
	endpoints map[string]controllerv1alpha1.EndpointList,
	routingObj RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {

	exposedEndpoints = map[string]controllerv1alpha1.ExposedEndpointList{}
	ready = true

	for machineName, machineEndpoints := range endpoints {
		for _, endpoint := range machineEndpoints {
			if endpoint.Exposure != controllerv1alpha1.PublicEndpointExposure {
				continue
			}
			endpointUrl, err := resolveURLForEndpoint(endpoint, routingObj)
			if err != nil {
				return nil, false, err
			}
			if endpointUrl == "" {
				ready = false
			}
			exposedEndpoints[machineName] = append(exposedEndpoints[machineName], controllerv1alpha1.ExposedEndpoint{
				Name:       endpoint.Name,
				Url:        endpointUrl,
				Attributes: endpoint.Attributes,
			})
		}
	}
	return exposedEndpoints, ready, nil
}

func resolveURLForEndpoint(
	endpoint controllerv1alpha1.Endpoint,
	routingObj RoutingObjects) (string, error) {
	for _, route := range routingObj.Routes {
		if route.Annotations[constants.DevWorkspaceEndpointNameAnnotation] == endpoint.Name {
			return getURLForEndpoint(endpoint, route.Spec.Host, route.Spec.Path, route.Spec.TLS != nil)
		}
	}
	for _, ingress := range routingObj.Ingresses {
		if ingress.Annotations[constants.DevWorkspaceEndpointNameAnnotation] == endpoint.Name {
			if len(ingress.Spec.Rules) == 1 {
				return getURLForEndpoint(endpoint, ingress.Spec.Rules[0].Host, "", false) // no TLS supported for ingresses yet
			} else {
				return "", fmt.Errorf("ingress %s contains multiple rules", ingress.Name)
			}
		}
	}
	return "", fmt.Errorf("could not find ingress/route for endpoint '%s'", endpoint.Name)
}

func getURLForEndpoint(endpoint controllerv1alpha1.Endpoint, host, basePath string, secure bool) (string, error) {
	protocol := endpoint.Protocol
	if secure && endpoint.Secure {
		protocol = controllerv1alpha1.EndpointProtocol(getSecureProtocol(string(protocol)))
	}

	// Format host/path ensuring only a single '/' character between the two. Can't use path.Join here as it would drop
	// a trailing '/' if present
	basehost := fmt.Sprintf("%s/%s", strings.TrimRight(host, "/"), strings.TrimLeft(basePath, "/"))
	baseUrl := fmt.Sprintf("%s://%s", protocol, basehost)

	url, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}

	if endpoint.Path != "" {
		relPath, err := url.Parse(endpoint.Path)
		if err != nil {
			return "", err
		}
		url = url.ResolveReference(relPath)
	}
	return url.String(), nil
}

// getSecureProtocol takes a (potentially unsecure protocol e.g. http) and returns the secure version (e.g. https).
// If protocol isn't recognized, it is returned unmodified.
func getSecureProtocol(protocol string) string {
	switch protocol {
	case "ws":
		return "wss"
	case "http":
		return "https"
	default:
		return protocol
	}
}
