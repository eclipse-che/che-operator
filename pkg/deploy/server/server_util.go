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
package server

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
)

func getComponentName(ctx *deploy.DeployContext) string {
	return deploy.DefaultCheFlavor(ctx.CheCluster)
}

func getOAuthConfig(ctx *deploy.DeployContext, oauthProvider string) (*corev1.Secret, error) {
	secrets, err := deploy.GetSecrets(ctx, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: oauthProvider,
	})

	if err != nil {
		return nil, err
	} else if len(secrets) == 0 {
		return nil, nil
	} else if len(secrets) > 1 {
		return nil, fmt.Errorf("More than 1 OAuth %s configuration secrets found", oauthProvider)
	}

	return &secrets[0], nil
}
