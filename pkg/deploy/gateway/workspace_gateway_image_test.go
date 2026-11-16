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
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/stretchr/testify/assert"
)

func TestGetWorkspaceGatewayImage(t *testing.T) {
	type testCase struct {
		name                    string
		gatewayDeploymentImages map[string]string
		gatewayContainer        *chev2.Container
		expectedImage           string
	}

	testCases := []testCase{
		{
			name:          "Default image",
			expectedImage: defaults.GetGatewayImage(&chev2.CheCluster{}),
		},
		{
			name:                    "Image from gateway deployment",
			gatewayDeploymentImages: map[string]string{constants.GatewayContainerName: "gateway:deployment"},
			expectedImage:           "gateway:deployment",
		},
		{
			name:                    "Image of another gateway deployment container is ignored",
			gatewayDeploymentImages: map[string]string{"oauth-proxy": "oauth-proxy:deployment"},
			expectedImage:           defaults.GetGatewayImage(&chev2.CheCluster{}),
		},
		{
			name:                    "devEnvironments gateway container override is not applied",
			gatewayDeploymentImages: map[string]string{constants.GatewayContainerName: "gateway:deployment"},
			gatewayContainer:        &chev2.Container{Image: "gateway:devenvironments"},
			expectedImage:           "gateway:deployment",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cheCluster := &chev2.CheCluster{}
			cheCluster.Spec.DevEnvironments.GatewayContainer = testCase.gatewayContainer

			if testCase.gatewayDeploymentImages != nil {
				cheCluster.Spec.Networking.Auth.Gateway.Deployment = &chev2.Deployment{}
				for name, image := range testCase.gatewayDeploymentImages {
					cheCluster.Spec.Networking.Auth.Gateway.Deployment.Containers = append(
						cheCluster.Spec.Networking.Auth.Gateway.Deployment.Containers,
						chev2.Container{Name: name, Image: image},
					)
				}
			}

			assert.Equal(t, testCase.expectedImage, GetWorkspaceGatewayImage(cheCluster))
		})
	}
}
