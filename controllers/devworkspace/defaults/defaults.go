//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"maps"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
)

const (
	configAnnotationPrefix                       = "che.routing.controller.devfile.io/"
	ConfigAnnotationDevWorkspaceRoutingName      = configAnnotationPrefix + "devworkspacerouting-name"
	ConfigAnnotationDevWorkspaceRoutingNamespace = configAnnotationPrefix + "devworkspacerouting-namespace"
	ConfigAnnotationEndpointName                 = configAnnotationPrefix + "endpoint-name"
	ConfigAnnotationComponentName                = configAnnotationPrefix + "component-name"
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

func GetGatewayWorkspaceConfigMapLabels(cluster *chev2.CheCluster) map[string]string {
	return maps.Clone(
		utils.DefaultIfEmpty(
			cluster.Spec.Networking.Auth.Gateway.ConfigLabels,
			constants.DefaultSingleHostGatewayConfigMapLabels,
		),
	)
}
