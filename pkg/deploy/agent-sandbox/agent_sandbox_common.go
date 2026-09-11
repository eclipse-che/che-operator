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

package agentsandbox

import (
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
)

func GetUserClusterRoleName() string {
	return defaults.GetCheFlavor() + "-user-agent-sandbox"
}

func GetUserRoleBindingName() string {
	return defaults.GetCheFlavor() + "-user-agent-sandbox"
}
