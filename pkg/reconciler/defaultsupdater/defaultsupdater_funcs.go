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

package defaultsupdater

import (
	"slices"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
)

// ensureChownCapability appends the `CHOWN` capability to
// `spec.devEnvironments.containerRunConfiguration.containerSecurityContext.capabilities.add`
// when a user has already configured a custom list of added capabilities.
// The capability is required for workspace containers to run on OpenShift.
// Reference issue: https://github.com/eclipse-che/che/issues/23748
func ensureChownCapability(cheCtx *chetypes.CheContext) error {
	runConfiguration := cheCtx.CheCluster.Spec.DevEnvironments.ContainerRunConfiguration
	if runConfiguration == nil ||
		runConfiguration.ContainerSecurityContext == nil ||
		runConfiguration.ContainerSecurityContext.Capabilities == nil {
		return nil
	}

	// Not default empty list.
	// Probably configured by a user.
	if len(runConfiguration.ContainerSecurityContext.Capabilities.Add) == 0 {
		return nil
	}

	if slices.Contains(runConfiguration.ContainerSecurityContext.Capabilities.Add, "CHOWN") {
		return nil
	}

	runConfiguration.ContainerSecurityContext.Capabilities.Add =
		append(runConfiguration.ContainerSecurityContext.Capabilities.Add, "CHOWN")

	return nil
}
