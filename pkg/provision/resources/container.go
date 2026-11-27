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

package resources

import (
	"slices"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
)

var Container = &containerHandler{}

type containerHandler struct{}

func (h *containerHandler) ApplyOverrides(container *corev1.Container, overrides *chev2.Container) {
	if overrides == nil {
		return
	}

	container.Image = utils.ValueOrDef(overrides.Image, container.Image)

	if overrides.ImagePullPolicy != "" {
		container.ImagePullPolicy = overrides.ImagePullPolicy
	} else if overrides.Image != "" || container.ImagePullPolicy == "" {
		container.ImagePullPolicy = corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(container.Image))
	}

	for _, overrideEnv := range overrides.Env {
		index := slices.IndexFunc(container.Env, func(e corev1.EnvVar) bool { return e.Name == overrideEnv.Name })
		if index == -1 {
			container.Env = append(container.Env, overrideEnv)
		} else {
			container.Env[index] = overrideEnv
		}
	}

	for _, vm := range overrides.VolumeMounts {
		index := slices.IndexFunc(container.VolumeMounts, func(m corev1.VolumeMount) bool { return m.Name == vm.Name })
		if index == -1 {
			container.VolumeMounts = append(container.VolumeMounts, vm)
		} else {
			container.VolumeMounts[index] = vm
		}
	}

	if overrides.Resources != nil {
		h.applyResourcesOverrides(&container.Resources.Requests, overrides.Resources.Requests)
		h.applyResourcesOverrides(&container.Resources.Limits, overrides.Resources.Limits)
	}

	if overrides.SecurityContext != nil {
		h.applySecurityContextOverrides(container, overrides.SecurityContext)
	}
}

// TrimCPULimitIfNoLimitRange removes the CPU limit of every container when the namespace
// LimitRange. It must be called before ApplyOverrides, so that a CPU limit overridden
// by the user is set afterwards and survives the trim.
// See details at https://github.com/eclipse/che/issues/22198
func (h *containerHandler) TrimCPULimitIfNoLimitRange(cheCtx *chetypes.CheContext, namespace string, container *corev1.Container) error {
	limitRanges, err := LimitRange.List(cheCtx, namespace)
	if err != nil {
		return err
	} else if len(limitRanges) == 0 {
		delete(container.Resources.Limits, corev1.ResourceCPU)
	}

	return nil
}

// applyResourcesOverrides merges the overridden memory and CPU values into the container
// resource list. A zero override value removes the entry, so users can drop a default
// request or limit by setting it to 0.
func (h *containerHandler) applyResourcesOverrides(list *corev1.ResourceList, overrides *chev2.ResourceList) {
	if overrides == nil {
		return
	}

	if *list == nil {
		*list = corev1.ResourceList{}
	}

	if overrides.Memory != nil {
		if overrides.Memory.IsZero() {
			delete(*list, corev1.ResourceMemory)
		} else {
			(*list)[corev1.ResourceMemory] = *overrides.Memory
		}
	}

	if overrides.Cpu != nil {
		if overrides.Cpu.IsZero() {
			delete(*list, corev1.ResourceCPU)
		} else {
			(*list)[corev1.ResourceCPU] = *overrides.Cpu
		}
	}

	if len(*list) == 0 {
		*list = nil
	}
}

func (h *containerHandler) applySecurityContextOverrides(container *corev1.Container, overrides *corev1.SecurityContext) {
	if overrides == nil {
		return
	}

	if container.SecurityContext == nil {
		container.SecurityContext = &corev1.SecurityContext{}
	}

	if overrides.Capabilities != nil {
		container.SecurityContext.Capabilities = overrides.Capabilities.DeepCopy()
	}
	if overrides.Privileged != nil {
		container.SecurityContext.Privileged = new(*overrides.Privileged)
	}
	if overrides.SELinuxOptions != nil {
		container.SecurityContext.SELinuxOptions = overrides.SELinuxOptions.DeepCopy()
	}
	if overrides.WindowsOptions != nil {
		container.SecurityContext.WindowsOptions = overrides.WindowsOptions.DeepCopy()
	}
	if overrides.RunAsUser != nil {
		container.SecurityContext.RunAsUser = new(*overrides.RunAsUser)
	}
	if overrides.RunAsGroup != nil {
		container.SecurityContext.RunAsGroup = new(*overrides.RunAsGroup)
	}
	if overrides.RunAsNonRoot != nil {
		container.SecurityContext.RunAsNonRoot = new(*overrides.RunAsNonRoot)
	}
	if overrides.ReadOnlyRootFilesystem != nil {
		container.SecurityContext.ReadOnlyRootFilesystem = new(*overrides.ReadOnlyRootFilesystem)
	}
	if overrides.AllowPrivilegeEscalation != nil {
		container.SecurityContext.AllowPrivilegeEscalation = new(*overrides.AllowPrivilegeEscalation)
	}
	if overrides.ProcMount != nil {
		container.SecurityContext.ProcMount = new(*overrides.ProcMount)
	}
	if overrides.SeccompProfile != nil {
		container.SecurityContext.SeccompProfile = overrides.SeccompProfile.DeepCopy()
	}
	if overrides.AppArmorProfile != nil {
		container.SecurityContext.AppArmorProfile = overrides.AppArmorProfile.DeepCopy()
	}
}
