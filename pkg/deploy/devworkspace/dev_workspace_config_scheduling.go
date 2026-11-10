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

package devworkspace

import (
	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	corev1 "k8s.io/api/core/v1"
)

func updateScheduling(cheCtx *chetypes.CheContext, workspaceConfig *controllerv1alpha1.WorkspaceConfig) {
	devEnvironments := &cheCtx.CheCluster.Spec.DevEnvironments

	workspaceConfig.TopologySpreadConstraints = nil
	workspaceConfig.PodAffinity = devEnvironments.PodAffinity.DeepCopy()
	workspaceConfig.PodAntiAffinity = devEnvironments.PodAntiAffinity.DeepCopy()

	if devEnvironments.TopologySpreadConstraints != nil {
		workspaceConfig.TopologySpreadConstraints = make([]corev1.TopologySpreadConstraint, len(devEnvironments.TopologySpreadConstraints))
		for i := range devEnvironments.TopologySpreadConstraints {
			devEnvironments.TopologySpreadConstraints[i].DeepCopyInto(&workspaceConfig.TopologySpreadConstraints[i])
		}
	}
}
