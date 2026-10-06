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

package openvsx_server

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/expose"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
)

func (r *OpenVSXServerReconciler) exposeEndpoint(cheCtx *chetypes.CheContext) (string, bool, error) {
	return expose.ExposeWithHostPath(
		cheCtx,
		constants.OpenVSXServerComponentName,
		"",
		"/"+constants.OpenVSXServerGatewayPath,
		r.createGatewayConfig(cheCtx))
}

func (r *OpenVSXServerReconciler) createGatewayConfig(cheCtx *chetypes.CheContext) *gateway.TraefikConfig {
	pathPrefix := "/" + constants.OpenVSXServerGatewayPath
	cfg := gateway.CreateCommonTraefikConfig(
		constants.OpenVSXServerComponentName,
		fmt.Sprintf("PathPrefix(`%s`)", pathPrefix),
		10,
		"http://"+constants.OpenVSXServerComponentName+":8080",
		[]string{})

	return cfg
}

func (r *OpenVSXServerReconciler) syncOpenVSXURLStatus(cheCtx *chetypes.CheContext) error {
	openVSXURL := "https://" + cheCtx.CheHost + "/" + constants.OpenVSXServerGatewayPath

	if openVSXURL != cheCtx.CheCluster.Status.OpenVSXURL {
		cheCtx.CheCluster.Status.OpenVSXURL = openVSXURL

		if err := deploy.UpdateCheCRStatus(cheCtx, "status: OpenVSXURL", openVSXURL); err != nil {
			return fmt.Errorf("failed to update status for OpenVSXURL: %w", err)
		}
	}

	return nil
}
