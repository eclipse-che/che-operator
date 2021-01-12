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
package deploy

import (
	"strings"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

func GetLegacyLabels(cheCluster *orgv1.CheCluster, component string) map[string]string {
	return map[string]string{
		"app":       DefaultCheFlavor(cheCluster),
		"component": component,
	}
}

func GetLabels(cheCluster *orgv1.CheCluster, component string) map[string]string {
	cheFlavor := DefaultCheFlavor(cheCluster)
	return map[string]string{
		KubernetesNameLabelKey:      cheFlavor,
		KubernetesInstanceLabelKey:  cheFlavor,
		KubernetesComponentLabelKey: component,
		KubernetesManagedByLabelKey: cheFlavor + "-operator",
	}
}

func GetLabelsAndSelector(cheCluster *orgv1.CheCluster, component string) (map[string]string, map[string]string) {
	labels := GetLabels(cheCluster, component)
	legacyLabels := GetLegacyLabels(cheCluster, component)

	// For the backward compatability
	// We have to keep these labels for a deployment since this field is immutable
	for k, v := range legacyLabels {
		labels[k] = v

	}

	return labels, legacyLabels
}

func MergeLabels(labels map[string]string, additionalLabels string) {
	for _, l := range strings.Split(additionalLabels, ",") {
		pair := strings.SplitN(l, "=", 2)
		if len(pair) == 2 {
			labels[pair[0]] = pair[1]
		}
	}
}
