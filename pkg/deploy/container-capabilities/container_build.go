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

package containercapabilties

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (r *ContainerBuild) getSCCSpec(sccName string) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   sccName,
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		AllowHostDirVolumePlugin: false,
		AllowHostIPC:             false,
		AllowHostNetwork:         false,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		AllowPrivilegeEscalation: pointer.Bool(true),
		AllowPrivilegedContainer: false,
		AllowedCapabilities:      []corev1.Capability{"SETUID", "SETGID"},
		DefaultAddCapabilities:   nil,
		FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs},
		ReadOnlyRootFilesystem:   false,
		RequiredDropCapabilities: []corev1.Capability{"KILL", "MKNOD"},
		RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAsRange},
		SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
		SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
		Users:                    []string{},
		Groups:                   []string{},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeDownwardAPI,
			securityv1.FSTypeEmptyDir,
			securityv1.FSTypePersistentVolumeClaim,
			securityv1.FSProjected,
			securityv1.FSTypeSecret,
		},
	}
}

func (r *ContainerBuild) getDWOClusterRoleName() string {
	return "dev-workspace-container-build"
}

func (r *ContainerBuild) getDWOClusterRoleBindingName() string {
	return "dev-workspace-container-build"
}

func (r *ContainerBuild) getFinalizer() string {
	return "container-build.finalizers.che.eclipse.org"
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
