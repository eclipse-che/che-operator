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
	"github.com/eclipse-che/che-operator/pkg/deploy"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type ContainerRun struct {
}

func NewContainerRun() *ContainerRun {
	return &ContainerRun{}
}

func (r *ContainerRun) getSCCSpec(sccName string) *securityv1.SecurityContextConstraints {
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
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type:   securityv1.FSGroupStrategyMustRunAs,
			Ranges: []securityv1.IDRange{{Min: 1000, Max: 65534}},
		},
		ReadOnlyRootFilesystem:   false,
		RequiredDropCapabilities: []corev1.Capability{"KILL", "MKNOD"},
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type: securityv1.RunAsUserStrategyMustRunAs,
			UID:  pointer.Int64(1000),
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type:           securityv1.SELinuxStrategyMustRunAs,
			SELinuxOptions: &corev1.SELinuxOptions{Type: "container_engine_t"},
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type:   securityv1.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityv1.IDRange{{Min: 1000, Max: 65534}},
		},
		Users:  []string{},
		Groups: []string{},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeDownwardAPI,
			securityv1.FSTypeEmptyDir,
			securityv1.FSTypePersistentVolumeClaim,
			securityv1.FSProjected,
			securityv1.FSTypeSecret,
		},
		UserNamespaceLevel: securityv1.NamespaceLevelRequirePod,
	}
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
