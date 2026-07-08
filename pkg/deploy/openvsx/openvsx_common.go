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

package openvsx

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
)

func GetOpenVSXServerServiceURL(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf("http://%s.%s.svc:%d/%s",
		constants.OpenVSXServerComponentName,
		ctx.CheCluster.Namespace,
		constants.OpenVSXServerServicePort,
		constants.OpenVSXServerGatewayPath,
	)
}
