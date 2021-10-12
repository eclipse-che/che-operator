//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package identity_provider

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

func GetWaitForKeycloakInitContainer(deployContext *deploy.DeployContext) (*corev1.Container, error) {
	keycloakEndpointCheckerImage := deploy.DefaultKeycloakImage(deployContext.CheCluster)
	imagePullPolicy := corev1.PullPolicy(deploy.DefaultPullPolicyFromDockerImage(keycloakEndpointCheckerImage))

	return &corev1.Container{
		Name:            "wait-for-keycloak",
		Image:           keycloakEndpointCheckerImage,
		ImagePullPolicy: imagePullPolicy,
		Args: []string{
			"/bin/sh",
			"-c",
			getCheckKeycloakReadinessScript(deployContext),
		},
	}, nil
}

func getCheckKeycloakReadinessScript(deployContext *deploy.DeployContext) string {
	cheFlavor := deploy.DefaultCheFlavor(deployContext.CheCluster)
	realmName := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
	// URL example: https://keycloak-eclipse-che.192.168.99.254.nip.io/auth/realms/che/.well-known/openid-configuration
	url := fmt.Sprintf("%s/realms/%s/.well-known/openid-configuration", deployContext.CheCluster.Status.KeycloakURL, realmName)

	return fmt.Sprintf(
		"until curl --connect-timeout 5 -kI %s; do echo 'waiting for Keycloak'; sleep 1; done;",
		url)
}
