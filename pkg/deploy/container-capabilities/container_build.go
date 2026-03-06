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

type ContainerBuild struct {
}

func NewContainerBuild() *ContainerBuild {
	return &ContainerBuild{}
}

func (r *ContainerBuild) GetUserRoleName() string {
	return defaults.GetCheFlavor() + "-user-container-build"
}

func (r *ContainerBuild) GetUserClusterRoleBindingName() string {
	return defaults.GetCheFlavor() + "-user-container-build"
}

func (r *ContainerBuild) applySCCSpec(scc *securityv1.SecurityContextConstraints) {
	scc.AllowHostDirVolumePlugin = false
	scc.AllowHostIPC = false
	scc.AllowHostNetwork = false
	scc.AllowHostPID = false
	scc.AllowHostPorts = false
	scc.AllowPrivilegeEscalation = pointer.Bool(true)
	scc.AllowPrivilegedContainer = false
	scc.AllowedCapabilities = []corev1.Capability{"SETUID", "SETGID"}
	scc.DefaultAddCapabilities = nil
	scc.FSGroup = securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs}
	scc.ReadOnlyRootFilesystem = false
	scc.RequiredDropCapabilities = []corev1.Capability{"KILL", "MKNOD"}
	scc.RunAsUser = securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAsRange}
	scc.SELinuxContext = securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs}
	scc.SupplementalGroups = securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny}
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
}

func (r *ContainerBuild) getDWOClusterRoleName() string {
	return "dev-workspace-container-build"
}

func (r *ContainerBuild) getDWOClusterRoleBindingName() string {
	return "dev-workspace-container-build"
}

func (r *ContainerBuild) getFinalizer() string {
	return constants.ContainerBuildFinalizer
}

func (r *ContainerBuild) getSCCName(cheCluster *chev2.CheCluster) string {
	if cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration != nil {
		return cheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint
	}

	return ""
}

func (r *ContainerBuild) getDefaultSCCName() string {
	return constants.DefaultContainerBuildSccName
}
