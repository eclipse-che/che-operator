//
// Copyright (c) 2020-2021 Red Hat, Inc.
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
	"os"
	"sort"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
)

func TestEnvVars(t *testing.T) {
	type testcase struct {
		name     string
		env      map[string]string
		expected []ImageAndName
	}

	// unset RELATED_IMAGE environment variables, set them back
	// after tests complete
	matches := util.GetEnvByRegExp("^RELATED_IMAGE_.*")
	for _, match := range matches {
		if originalValue, exists := os.LookupEnv(match.Name); exists {
			os.Unsetenv(match.Name)
			defer os.Setenv(match.Name, originalValue)
		}
	}

	cases := []testcase{
		{
			name: "detect plugin broker images",
			env: map[string]string{
				"RELATED_IMAGE_che_workspace_plugin_broker_artifacts": "quay.io/eclipse/che-plugin-metadata-broker",
				"RELATED_IMAGE_che_workspace_plugin_broker_metadata":  "quay.io/eclipse/che-plugin-artifacts-broker",
			},
			expected: []ImageAndName{
				{Name: "che_workspace_plugin_broker_artifacts", Image: "quay.io/eclipse/che-plugin-metadata-broker"},
				{Name: "che_workspace_plugin_broker_metadata", Image: "quay.io/eclipse/che-plugin-artifacts-broker"},
			},
		},
		{
			name: "detect theia images",
			env: map[string]string{
				"RELATED_IMAGE_che_theia_plugin_registry_image_IBZWQYJ":                         "quay.io/eclipse/che-theia",
				"RELATED_IMAGE_che_theia_endpoint_runtime_binary_plugin_registry_image_IBZWQYJ": "quay.io/eclipse/che-theia-endpoint-runtime-binary",
			},
			expected: []ImageAndName{
				{Name: "che_theia_plugin_registry_image_IBZWQYJ", Image: "quay.io/eclipse/che-theia"},
				{Name: "che_theia_endpoint_runtime_binary_plugin_registry_image_IBZWQYJ", Image: "quay.io/eclipse/che-theia-endpoint-runtime-binary"},
			},
		},
		{
			name: "detect machine exec image",
			env: map[string]string{
				"RELATED_IMAGE_che_machine_exec_plugin_registry_image_IBZWQYJ":                  "quay.io/eclipse/che-machine-exec",
				"RELATED_IMAGE_codeready_workspaces_machineexec_plugin_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/machineexec-rhel8",
			},
			expected: []ImageAndName{
				{Name: "che_machine_exec_plugin_registry_image_IBZWQYJ", Image: "quay.io/eclipse/che-machine-exec"},
				{Name: "codeready_workspaces_machineexec_plugin_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/machineexec-rhel8"},
			},
		},
		{
			name: "detect plugin registry images",
			env: map[string]string{
				"RELATED_IMAGE_che_openshift_plugin_registry_image_IBZWQYJ":                          "index.docker.io/dirigiblelabs/dirigible-openshift",
				"RELATED_IMAGE_codeready_workspaces_plugin_openshift_plugin_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/plugin-openshift-rhel8",
			},
			expected: []ImageAndName{
				{Name: "che_openshift_plugin_registry_image_IBZWQYJ", Image: "index.docker.io/dirigiblelabs/dirigible-openshift"},
				{Name: "codeready_workspaces_plugin_openshift_plugin_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/plugin-openshift-rhel8"},
			},
		},
		{
			name: "detect devfile registry images",
			env: map[string]string{
				"RELATED_IMAGE_che_cpp_rhel7_devfile_registry_image_G4XDGNR":                       "quay.io/eclipse/che-cpp-rhel7",
				"RELATED_IMAGE_che_dotnet_2_2_devfile_registry_image_G4XDGNR":                      "quay.io/eclipse/che-dotnet-2.2",
				"RELATED_IMAGE_che_dotnet_3_1_devfile_registry_image_G4XDGNR":                      "quay.io/eclipse/che-dotnet-3.1",
				"RELATED_IMAGE_che_golang_1_14_devfile_registry_image_G4XDGNR":                     "quay.io/eclipse/che-golang-1.14",
				"RELATED_IMAGE_che_php_7_devfile_registry_image_G4XDGNR":                           "quay.io/eclipse/che-php-7",
				"RELATED_IMAGE_che_java11_maven_devfile_registry_image_G4XDGNR":                    "quay.io/eclipse/che-java11-maven",
				"RELATED_IMAGE_che_java8_maven_devfile_registry_image_G4XDGNR":                     "quay.io/eclipse/che-java8-maven",
				"RELATED_IMAGE_codeready_workspaces_stacks_cpp_devfile_registry_image_GIXDCMQK":    "registry.redhat.io/codeready-workspaces/stacks-cpp-rhel8",
				"RELATED_IMAGE_codeready_workspaces_stacks_dotnet_devfile_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/stacks-dotnet-rhel8",
				"RELATED_IMAGE_codeready_workspaces_stacks_golang_devfile_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/stacks-golang-rhel8",
				"RELATED_IMAGE_codeready_workspaces_stacks_php_devfile_registry_image_GIXDCMQK":    "registry.redhat.io/codeready-workspaces/stacks-php-rhel8",
				"RELATED_IMAGE_codeready_workspaces_plugin_java11_devfile_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/plugin-java11-rhel8",
				"RELATED_IMAGE_codeready_workspaces_plugin_java8_devfile_registry_image_GIXDCMQK":  "registry.redhat.io/codeready-workspaces/plugin-java8-rhel8",
			},
			expected: []ImageAndName{
				{Name: "che_cpp_rhel7_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-cpp-rhel7"},
				{Name: "che_dotnet_2_2_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-dotnet-2.2"},
				{Name: "che_dotnet_3_1_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-dotnet-3.1"},
				{Name: "che_golang_1_14_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-golang-1.14"},
				{Name: "che_php_7_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-php-7"},
				{Name: "che_java11_maven_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-java11-maven"},
				{Name: "che_java8_maven_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-java8-maven"},
				{Name: "codeready_workspaces_stacks_cpp_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-cpp-rhel8"},
				{Name: "codeready_workspaces_stacks_dotnet_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-dotnet-rhel8"},
				{Name: "codeready_workspaces_stacks_golang_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-golang-rhel8"},
				{Name: "codeready_workspaces_stacks_php_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-php-rhel8"},
				{Name: "codeready_workspaces_plugin_java11_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/plugin-java11-rhel8"},
				{Name: "codeready_workspaces_plugin_java8_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/plugin-java8-rhel8"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			actual := GetDefaultImages()
			if d := cmp.Diff(sortImages(c.expected), sortImages(actual)); d != "" {
				t.Errorf("Error, collected images differ (-want, +got): %v", d)
			}
		})
	}
}

func sortImages(images []ImageAndName) []ImageAndName {
	imagesCopy := make([]ImageAndName, len(images))
	copy(imagesCopy, images)
	sort.Slice(imagesCopy, func(i, j int) bool {
		return imagesCopy[i].Name < imagesCopy[j].Name
	})
	return imagesCopy
}
