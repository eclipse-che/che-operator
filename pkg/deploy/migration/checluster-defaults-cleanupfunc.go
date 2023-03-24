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
	"fmt"
	"regexp"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"github.com/google/go-cmp/cmp/cmpopts"

	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"golang.org/x/mod/semver"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/pointer"
)

// cleanUpDevEnvironmentsDefaultEditor cleans up CheCluster CR `Spec.DevEnvironments.DefaultEditor`.
// A new default is set via environment variable `CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTEDITOR`.
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

// cleanUpDevEnvironmentsDefaultComponents cleans up CheCluster CR `Spec.DevEnvironments.DefaultComponents`.
// A new default is set via environment variable `CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTCOMPONENTS`.
func cleanUpDevEnvironmentsDefaultComponents(ctx *chetypes.DeployContext) (bool, error) {
	devEnvironmentsDefaultComponents := []string{
		"[{\"name\": \"universal-developer-image\", \"container\": {\"image\": \"quay.io/devfile/universal-developer-image:ubi8-38da5c2\"}}]", // previous default
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
			ctx.CheCluster.Spec.DevEnvironments.DefaultComponents = []devfile.Component{}
			return true, nil
		}
	}

	return false, nil
}

// cleanUpDashboardHeaderMessage cleans up CheCluster CR `Spec.Components.Dashboard.HeaderMessage`.
// A new default is set via environment variable `CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT`.
func cleanUpDashboardHeaderMessage(ctx *chetypes.DeployContext) (bool, error) {
	dashboardHeaderMessageTextRegExp := []string{
		defaults.GetDashboardHeaderMessageText(),
	}

	for _, textRegExp := range dashboardHeaderMessageTextRegExp {
		if ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage != nil {
			rxp := regexp.MustCompile(textRegExp)
			if rxp.MatchString(ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage.Text) {
				ctx.CheCluster.Spec.Components.Dashboard.HeaderMessage = nil
				return true, nil
			}
		}
	}

	return false, nil
}

// cleanUpPluginRegistryOpenVSXURL cleans up CheCluster CR `Spec.Components.PluginRegistry.OpenVSXURL`.
// The logic of Open VSX Registry usage depends on the `Spec.Components.PluginRegistry.OpenVSXURL` value:
//   * <not set>, uses environment variable `CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL` as the URL
//     for the registry (default value)
//   * <empty>, starts embedded Open VSX Registry
//   * <non-empty>, uses as the URL for the registry
// Clean up is done in the following way (complies with requirements https://github.com/eclipse/che/issues/21637):
//   * if Eclipse Che is being installed, then use the default openVSXURL
//   * if Eclipse Che is being upgraded
//     - if value is <not set> and Eclipse Che v7.52 or earlier, then set the default
//     - if value is <not set> and Eclipse Che v7.53 or later, then set it to empty string (starts embedded registry)
//     - if value is <empty>, then do nothing (starts embedded registry)
//     - if value is <non-empty> and equals to the default value, then set it to nil since default is moved to environment variable
//       (use external registry from the environment variable)
//     - if value is <non-empty> and not equals to the default value, then do nothing (use external registry from the value)
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

	// Eclipse Che is being installed
	if ctx.CheCluster.Status.CheVersion == "" {
		if ctx.CheCluster.IsAirGapMode() {
			ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL = pointer.StringPtr("")
			return true, nil
		}

		return false, nil
	}

	// Eclipse Che is being upgraded
	if ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL == nil &&
		ctx.CheCluster.IsCheFlavor() &&
		ctx.CheCluster.Status.CheVersion != "" &&
		ctx.CheCluster.Status.CheVersion != "next" &&
		semver.Compare(fmt.Sprintf("v%s", ctx.CheCluster.Status.CheVersion), "v7.53.0") == -1 {
		// Eclipse Che is being updated from version v7.52 or earlier
		ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL = pointer.StringPtr(defaults.GetPluginRegistryOpenVSXURL())
		return false, nil
	}

	if ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL == nil {
		ctx.CheCluster.Spec.Components.PluginRegistry.OpenVSXURL = pointer.StringPtr("")
		return true, nil
	}

	return false, nil
}

// cleanUpDevEnvironmentsDisableContainerBuildCapabilities cleans up CheCluster CR `Spec.DevEnvironments.DisableContainerBuildCapabilities`.
// A new default is set via environment variable `CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DISABLECONTAINERBUILDCAPABILITIES`.
func cleanUpDevEnvironmentsDisableContainerBuildCapabilities(ctx *chetypes.DeployContext) (bool, error) {
	if !infrastructure.IsOpenShift() {
		ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = pointer.BoolPtr(true)
		return true, nil
	}

	ctx.CheCluster.Spec.DevEnvironments.DisableContainerBuildCapabilities = nil
	return true, nil
}
