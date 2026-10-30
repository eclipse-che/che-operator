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

package chelabels

import (
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
)

func GetStandardLabels(component string) map[string]string {
	return map[string]string{
		constants.KubernetesComponentLabelKey: component,
		constants.KubernetesManagedByLabelKey: defaults.GetOperatorName(),
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesNameLabelKey:      defaults.GetCheFlavor(),
		constants.KubernetesInstanceLabelKey:  defaults.GetCheFlavor(),
	}
}

func GetOperatorManagedLabels(component string) map[string]string {
	return map[string]string{
		constants.KubernetesComponentLabelKey: component,
		constants.KubernetesManagedByLabelKey: defaults.GetOperatorName(),
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
	}
}
