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

package gateway

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
)

// GetWorkspaceGatewayImage returns the image of the gateway sidecar that is added to every workspace pod,
// before the spec.devEnvironments.gatewayContainer overrides are applied.
func GetWorkspaceGatewayImage(cheCluster *chev2.CheCluster) string {
	image := defaults.GetGatewayImage(cheCluster)

	if cheCluster.Spec.Networking.Auth.Gateway.Deployment != nil {
		for _, container := range cheCluster.Spec.Networking.Auth.Gateway.Deployment.Containers {
			if container.Name == constants.GatewayContainerName {
				image = utils.GetValue(container.Image, image)
			}
		}
	}

	return image
}
