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
package deploy

import (
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
)

func TestLabels(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheFlavor: "che",
			},
		},
	}

	labels, _ := GetLabelsAndSelector(cheCluster, "test")
	if labels[KubernetesNameLabelKey] == "" ||
		labels[KubernetesComponentLabelKey] == "" ||
		labels[KubernetesInstanceLabelKey] == "" ||
		labels[KubernetesManagedByLabelKey] == "" {
		t.Errorf("Default kubernetes labels aren't set.")
	}
}
