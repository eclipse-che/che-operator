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

func GetLabels(cr *orgv1.CheCluster, component string) (labels map[string]string) {
	cheFlavor := DefaultCheFlavor(cr)
	labels = map[string]string{
		KubernetesNameLabelKey:      cheFlavor,
		KubernetesPartOfLabelKey:    CheEclipseOrg,
		KubernetesInstanceLabelKey:  cheFlavor + "-" + cr.Namespace,
		KubernetesManagedByLabelKey: cheFlavor + "-operator",
		KubernetesComponentLabelKey: component,

		// for backward compatability
		"app":       cheFlavor,
		"component": component,
	}
	return labels
}

func MergeLabels(labels map[string]string, additionalLabels string) {
	for _, l := range strings.Split(additionalLabels, ",") {
		pair := strings.SplitN(l, "=", 2)
		if len(pair) == 2 {
			labels[pair[0]] = pair[1]
		}
	}
}
