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

package specmigration

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
)

// addChownCapability appends the `CHOWN` capability to
// `spec.devEnvironments.containerRunConfiguration.containerSecurityContext.capabilities.add`.
// The capability is required for workspace containers to run on OpenShift.
// Reference issue: https://github.com/eclipse-che/che/issues/23748
func addChownCapability(cheCtx *chetypes.CheContext) error {
	runConfiguration := cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration
	if runConfiguration == nil ||
		runConfiguration.ContainerSecurityContext == nil ||
		runConfiguration.ContainerSecurityContext.Capabilities == nil {
		return nil
	}

	// Do nothing, it is overridden by a user.
	if len(runConfiguration.ContainerSecurityContext.Capabilities.Add) == 0 {
		return nil
	}

	runConfiguration.ContainerSecurityContext.Capabilities.Add = utils.AppendUnique(
		runConfiguration.ContainerSecurityContext.Capabilities.Add,
		"CHOWN",
	)
	return nil
}
