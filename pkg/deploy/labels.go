//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package deploy

import (
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
)

func GetLabels(component string) map[string]string {
	return map[string]string{
		constants.KubernetesNameLabelKey:      defaults.GetCheFlavor(),
		constants.KubernetesInstanceLabelKey:  defaults.GetCheFlavor(),
		constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
		constants.KubernetesComponentLabelKey: component,
		constants.KubernetesManagedByLabelKey: GetManagedByLabel(),
	}
}

func GetManagedByLabel() string {
	return defaults.GetCheFlavor() + "-operator"
}

func GetLabelsAndSelector(component string) (map[string]string, map[string]string) {
	labels := GetLabels(component)
	legacyLabels := GetLegacyLabels(component)

	// For the backward compatability
	// We have to keep these labels for a deployment since this field is immutable
	for k, v := range legacyLabels {
		labels[k] = v
	}

	return labels, legacyLabels
}

func GetLegacyLabels(component string) map[string]string {
	return map[string]string{
		"app":       defaults.GetCheFlavor(),
		"component": component,
	}
}

func IsPartOfEclipseCheResourceAndManagedByOperator(labels map[string]string) bool {
	return labels[constants.KubernetesPartOfLabelKey] == constants.CheEclipseOrg && labels[constants.KubernetesManagedByLabelKey] == GetManagedByLabel()
}
