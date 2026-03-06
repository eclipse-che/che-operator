//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package containercapabilities

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

type ContainerRun struct {
}

func NewContainerRun() *ContainerRun {
	return &ContainerRun{}
}

func (r *ContainerRun) applySCCSpec(scc *securityv1.SecurityContextConstraints) {
	scc.AllowHostDirVolumePlugin = false
	scc.AllowHostIPC = false
	scc.AllowHostNetwork = false
	scc.AllowHostPID = false
	scc.AllowHostPorts = false
	scc.AllowPrivilegeEscalation = pointer.Bool(true)
	scc.AllowPrivilegedContainer = false
	scc.AllowedCapabilities = []corev1.Capability{"SETUID", "SETGID", "CHOWN"}
	scc.DefaultAddCapabilities = nil
	scc.FSGroup = securityv1.FSGroupStrategyOptions{
		Type:   securityv1.FSGroupStrategyMustRunAs,
		Ranges: []securityv1.IDRange{{Min: 1000, Max: 65534}},
	}
	scc.ReadOnlyRootFilesystem = false
	scc.RequiredDropCapabilities = []corev1.Capability{"KILL", "MKNOD"}
	scc.RunAsUser = securityv1.RunAsUserStrategyOptions{
		Type: securityv1.RunAsUserStrategyMustRunAs,
		UID:  pointer.Int64(1000),
	}
	scc.SELinuxContext = securityv1.SELinuxContextStrategyOptions{
		Type:           securityv1.SELinuxStrategyMustRunAs,
		SELinuxOptions: &corev1.SELinuxOptions{Type: "container_engine_t"},
	}
	scc.SupplementalGroups = securityv1.SupplementalGroupsStrategyOptions{
		Type:   securityv1.SupplementalGroupsStrategyMustRunAs,
		Ranges: []securityv1.IDRange{{Min: 1000, Max: 65534}},
	}
	scc.Users = []string{}
	scc.Groups = []string{}
	scc.Volumes = []securityv1.FSType{
		securityv1.FSTypeConfigMap,
		securityv1.FSTypeDownwardAPI,
		securityv1.FSTypeEmptyDir,
		securityv1.FSTypePersistentVolumeClaim,
		securityv1.FSProjected,
		securityv1.FSTypeSecret,
	}
	scc.UserNamespaceLevel = securityv1.NamespaceLevelRequirePod
}

func (r *ContainerRun) GetUserRoleName() string {
	return defaults.GetCheFlavor() + "-user-container-run"
}

func (r *ContainerRun) GetUserClusterRoleBindingName() string {
	return defaults.GetCheFlavor() + "-user-container-run"
}

func (r *ContainerRun) getDWOClusterRoleBindingName() string {
	return "dev-workspace-container-run"
}

func (r *ContainerRun) getDWOClusterRoleName() string {
	return "dev-workspace-container-run"
}

func (r *ContainerRun) getFinalizer() string {
	return constants.ContainerRunFinalizer
}

func (r *ContainerRun) getSCCName(cheCluster *chev2.CheCluster) string {
	if cheCluster.Spec.DevEnvironments.ContainerRunConfiguration != nil {
		return cheCluster.Spec.DevEnvironments.ContainerRunConfiguration.OpenShiftSecurityContextConstraint
	}

	return ""
}

func (r *ContainerRun) getDefaultSCCName() string {
	return constants.DefaultContainerRunSccName
}
