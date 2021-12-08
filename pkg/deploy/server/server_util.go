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
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"github.com/eclipse-che/che-operator/pkg/util"
)

func getComponentName(ctx *deploy.DeployContext) string {
	return deploy.DefaultCheFlavor(ctx.CheCluster)
}

func getServerExposingServiceName(cr *orgv1.CheCluster) string {
	if util.GetServerExposureStrategy(cr) == "single-host" && deploy.GetSingleHostExposureType(cr) == deploy.GatewaySingleHostExposureType {
		return gateway.GatewayServiceName
	}
	return deploy.CheServiceName
}
