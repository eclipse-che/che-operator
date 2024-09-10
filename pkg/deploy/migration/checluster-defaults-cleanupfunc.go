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

package migration

import (
	"encoding/json"
	"strconv"

	chev2 "github.com/eclipse-che/che-operator/api/v2"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"github.com/google/go-cmp/cmp/cmpopts"

	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"
)

// cleanUpDevEnvironmentsDefaultEditor cleans up CheCluster CR `Spec.DevEnvironments.DefaultEditor` field.
func cleanUpDevEnvironmentsDefaultEditor(ctx *chetypes.DeployContext) (bool, error) {
	devEnvironmentsDefaultEditor := []string{
		"eclipse/che-theia/latest",                 // is not supported anymore, see details at https://github.com/eclipse/che/issues/21771
		"che-incubator/che-code/insiders",          // is replaced by `che-incubator/che-code/latest`, see details at https://issues.redhat.com/browse/CRW-3568
		"che-incubator/che-code/latest",            // previous default
		defaults.GetDevEnvironmentsDefaultEditor(), // current default (can be equal to the previous one)
	}

	for _, defaultEditor := range devEnvironmentsDefaultEditor {
		if ctx.CheCluster.Spec.DevEnvironments.DefaultEditor == defaultEditor {
			ctx.CheCluster.Spec.DevEnvironments.DefaultEditor = ""
			return true, nil
		}
	}

	return false, nil
}

// cleanUpDevEnvironmentsDefaultComponents cleans up CheCluster CR `Spec.DevEnvironments.DefaultComponents` field.
func cleanUpDevEnvironmentsDefaultComponents(ctx *chetypes.DeployContext) (bool, error) {
	devEnvironmentsDefaultComponents := []string{
		"[{\"name\": \"universal-developer-image\", " +
			"\"container\": {\"image\": \"quay.io/devfile/universal-developer-image:ubi8-38da5c2\"}}]", // previous default
		defaults.GetDevEnvironmentsDefaultComponents(), // current default (can be equal to the previous one)
	}

	for _, defaultComponentStr := range devEnvironmentsDefaultComponents {
		var defaultComponent []devfile.Component
		if err := json.Unmarshal([]byte(defaultComponentStr), &defaultComponent); err != nil {
			return false, err
		}

		if cmp.Diff(
			defaultComponent,
			ctx.CheCluster.Spec.DevEnvironments.DefaultComponents,
			cmp.Options{
				cmpopts.IgnoreFields(devfile.Container{}, "SourceMapping"), // SourceMapping can have a default value, so it should be ignored
			}) == "" {
			ctx.CheCluster.Spec.DevEnvironments.DefaultComponents = nil
			return true, nil
		}
	}

	return false, nil
}

// cleanUpDashboardHeaderMessage cleans up CheCluster CR `Spec.Components.Dashboard.HeaderMessage`.
func cleanUpDashboardHeaderMessage(ctx *chetypes.DeployContext) (bool, error) {
	dashboardHeaderMessageText := []string{
		defaults.GetDashboardHeaderMessageText(),
	}

	if ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage != nil {
		for _, text := range dashboardHeaderMessageText {
			if ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage.Text == text {
				ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage = nil
				return true, nil
			}
		}
	}

	return false, nil
}

// cleanUpPluginRegistryOpenVSXURL cleans up CheCluster CR `Spec.Components.PluginRegistry.OpenVSXURL` field.
func cleanUpPluginRegistryOpenVSXURL(ctx *chetypes.DeployContext) (bool, error) {
	pluginRegistryOpenVSXURL := []string{
		"https://openvsx.org",                  // redirects to "https://open-vsx.org"
		"https://open-vsx.org",                 // previous default
		defaults.GetPluginRegistryOpenVSXURL(), // current default (can be equal to the previous one)
	}

	if ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL != nil {
		for _, openVSXURL := range pluginRegistryOpenVSXURL {
			if *ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL == openVSXURL {
				ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL = nil
				return true, nil
			}
		}
	}

	return false, nil
}

// cleanUpDevEnvironmentsDisableContainerBuildCapabilities cleans up
// CheCluster CR `Spec.DevEnvironments.DisableContainerBuildCapabilities` field. See also [v2.CheCluster].
func cleanUpDevEnvironmentsDisableContainerBuildCapabilities(ctx *chetypes.DeployContext) (bool, error) {
	if !infrastructure.IsOpenShift() {
		ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(true)
		return true, nil
	}

	if ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities != nil {
		disableContainerBuildCapabilities, err := strconv.ParseBool(defaults.GetDevEnvironmentsDisableContainerBuildCapabilities())
		if err != nil {
			return false, err
		}
		if disableContainerBuildCapabilities == *ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities {
			ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = nil
			return true, nil
		}
	}

	return false, nil
}

func cleanUpContainersResources(ctx *chetypes.DeployContext) (bool, error) {
	deployments := []*chev2.Deployment{
		ctx.CheCluster.Spec.Components.CheServer.Deployment,
		ctx.CheCluster.Spec.Components.PluginRegistry.Deployment,
		ctx.CheCluster.Spec.Components.Dashboard.Deployment,
		ctx.CheCluster.Spec.Networking.Auth.Gateway.Deployment,
	}

	done := false
	for _, deployment := range deployments {
		if deployment != nil {
			for _, container := range deployment.Containers {
				if container.Resources != nil {
					if container.Resources.Requests != nil {
						if container.Resources.Requests.Memory != nil && container.Resources.Requests.Memory.IsZero() {
							container.Resources.Requests.Memory = nil
							done = true
						}

						if container.Resources.Requests.Cpu != nil && container.Resources.Requests.Cpu.IsZero() {
							container.Resources.Requests.Cpu = nil
							done = true
						}

						if container.Resources.Limits.Memory != nil && container.Resources.Limits.Memory.IsZero() {
							container.Resources.Limits.Memory = nil
							done = true
						}

						if container.Resources.Limits.Cpu != nil && container.Resources.Limits.Cpu.IsZero() {
							container.Resources.Limits.Cpu = nil
							done = true
						}
					}
				}
			}
		}
	}

	return done, nil
}
