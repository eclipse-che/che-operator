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

package defaults

import (
	"os"
	"runtime"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	gatewayImageEnvVarName           = "RELATED_IMAGE_gateway"
	gatewayConfigurerImageEnvVarName = "RELATED_IMAGE_gateway_configurer"

	defaultGatewayImage           = "quay.io/eclipse/che--traefik:v2.5.0-eb30f9f09a65cee1fab5ef9c64cb4ec91b800dc3fdd738d62a9d4334f0114683"
	defaultGatewayConfigurerImage = "quay.io/che-incubator/configbump:0.1.4"

	configAnnotationPrefix                       = "che.routing.controller.devfile.io/"
	ConfigAnnotationCheManagerName               = configAnnotationPrefix + "che-name"
	ConfigAnnotationCheManagerNamespace          = configAnnotationPrefix + "che-namespace"
	ConfigAnnotationDevWorkspaceRoutingName      = configAnnotationPrefix + "devworkspacerouting-name"
	ConfigAnnotationDevWorkspaceRoutingNamespace = configAnnotationPrefix + "devworkspacerouting-namespace"
	ConfigAnnotationEndpointName                 = configAnnotationPrefix + "endpoint-name"
	ConfigAnnotationComponentName                = configAnnotationPrefix + "component-name"
)

var (
	log = ctrl.Log.WithName("defaults")

	DefaultIngressAnnotations = map[string]string{
		"kubernetes.io/ingress.class":                       "nginx",
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
		"nginx.ingress.kubernetes.io/ssl-redirect":          "true",
	}

	// If this looks weirdly out of place to you from all other labels, then you're completely right!
	// These labels are the default ones used by che-operator and Che7. Let's keep the defaults
	// the same for the ease of translation...
	defaultGatewayConfigLabels = map[string]string{
		"app":       "che",
		"component": "che-gateway-config",
	}
)

func GetGatewayWorkspaceConfigMapName(workspaceID string) string {
	return workspaceID + "-route"
}

func GetLabelsForComponent(cluster *chev2.CheCluster, component string) map[string]string {
	return GetLabelsFromNames(cluster.Name, component)
}

func GetLabelsFromNames(appName string, component string) map[string]string {
	return AddStandardLabelsFromNames(appName, component, map[string]string{})
}

func AddStandardLabelsForComponent(cluster *chev2.CheCluster, component string, labels map[string]string) map[string]string {
	return AddStandardLabelsFromNames(cluster.Name, component, labels)
}

func AddStandardLabelsFromNames(appName string, component string, labels map[string]string) map[string]string {
	labels["app.kubernetes.io/name"] = appName
	labels["app.kubernetes.io/part-of"] = constants.CheEclipseOrg
	labels["app.kubernetes.io/component"] = component
	return labels
}

func GetGatewayImage() string {
	return read(gatewayImageEnvVarName, defaultGatewayImage)
}

func GetGatewayConfigurerImage() string {
	return read(gatewayConfigurerImageEnvVarName, defaultGatewayConfigurerImage)
}

func GetIngressAnnotations(cluster *chev2.CheCluster) map[string]string {
	if len(cluster.Spec.Ingress.Annotations) > 0 {
		return cluster.Spec.Ingress.Annotations
	}
	return DefaultIngressAnnotations
}

func GetGatewayWorkspaceConfigMapLabels(cluster *chev2.CheCluster) map[string]string {
	if len(cluster.Spec.Ingress.Auth.Gateway.ConfigLabels) > 0 {
		return cluster.Spec.Ingress.Auth.Gateway.ConfigLabels
	}
	return defaultGatewayConfigLabels
}

func read(varName string, fallback string) string {
	ret := os.Getenv(varName)

	if len(ret) == 0 {
		ret = os.Getenv(archDependent(varName))
		if len(ret) == 0 {
			log.Info("Failed to read the default value from the environment. Will use the hardcoded default value.", "envvar", varName, "value", fallback)
			ret = fallback
		}
	}

	return ret
}

func archDependent(envVarName string) string {
	return envVarName + "_" + runtime.GOARCH
}
