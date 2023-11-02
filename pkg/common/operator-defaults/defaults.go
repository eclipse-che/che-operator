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

package defaults

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	util "github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
)

var (
	defaultCheVersion                                       string
	defaultCheFlavor                                        string
	defaultCheServerImage                                   string
	defaultDashboardImage                                   string
	defaultPluginRegistryImage                              string
	defaultDevfileRegistryImage                             string
	defaultCheTLSSecretsCreationJobImage                    string
	defaultSingleHostGatewayImage                           string
	defaultSingleHostGatewayConfigSidecarImage              string
	defaultGatewayAuthenticationSidecarImage                string
	defaultGatewayAuthorizationSidecarImage                 string
	defaultConsoleLinkName                                  string
	defaultConsoleLinkDisplayName                           string
	defaultConsoleLinkSection                               string
	defaultsConsoleLinkImage                                string
	defaultDevEnvironmentsDefaultEditor                     string
	defaultDevEnvironmentsDefaultComponents                 string
	defaultDevEnvironmentsDisableContainerBuildCapabilities string
	defaultDevEnvironmentsContainerSecurityContext          string
	defaultPluginRegistryOpenVSXURL                         string
	defaultDashboardHeaderMessageText                       string

	initialized = false
)

func InitializeForTesting(operatorDeploymentFilePath string) {
	operatorDeployment := &appsv1.Deployment{}
	if err := util.ReadObjectInto(operatorDeploymentFilePath, operatorDeployment); err != nil {
		logrus.Fatalf("Failed to read operator deployment from '%s', cause: %v", operatorDeploymentFilePath, err)
	}

	for _, container := range operatorDeployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			os.Setenv(env.Name, env.Value)
		}
	}

	Initialize()
}

func Initialize() {
	defaultCheVersion = ensureEnv("CHE_VERSION")
	defaultCheFlavor = ensureEnv("CHE_FLAVOR")
	defaultConsoleLinkDisplayName = ensureEnv("CONSOLE_LINK_DISPLAY_NAME")
	defaultConsoleLinkName = ensureEnv("CONSOLE_LINK_NAME")
	defaultConsoleLinkSection = ensureEnv("CONSOLE_LINK_SECTION")
	defaultsConsoleLinkImage = ensureEnv("CONSOLE_LINK_IMAGE")

	defaultDevEnvironmentsDisableContainerBuildCapabilities = ensureEnv("CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DISABLECONTAINERBUILDCAPABILITIES")
	defaultDevEnvironmentsContainerSecurityContext = ensureEnv(("CHE_DEFAULT_SPEC_DEVENVIRONMENTS_CONTAINERSECURITYCONTEXT"))
	defaultDevEnvironmentsDefaultComponents = ensureEnv("CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTCOMPONENTS")

	// can be empty
	defaultDevEnvironmentsDefaultEditor = os.Getenv("CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTEDITOR")
	defaultPluginRegistryOpenVSXURL = os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL")
	defaultDashboardHeaderMessageText = os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT")

	defaultCheServerImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_che_server"))
	defaultDashboardImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_dashboard"))
	defaultPluginRegistryImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_devfile_registry"))
	defaultSingleHostGatewayImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_single_host_gateway_config_sidecar"))
	defaultGatewayAuthenticationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authentication_sidecar"))
	defaultGatewayAuthorizationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authorization_sidecar"))

	// Don't get some k8s specific env
	if !infrastructure.IsOpenShift() {
		defaultCheTLSSecretsCreationJobImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_che_tls_secrets_creation_job"))
		defaultGatewayAuthenticationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authentication_sidecar_k8s"))
		defaultGatewayAuthorizationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authorization_sidecar_k8s"))
	}

	initialized = true
}

func ensureEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		logrus.Fatalf("Failed to initialize default value: '%s'. Environment variable not found.", name)
	}

	return value
}

func GetCheServerImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultCheServerImage)
}

func GetCheTLSSecretsCreationJobImage() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultCheTLSSecretsCreationJobImage
}

func GetCheVersion() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultCheVersion
}

func GetDashboardImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultDashboardImage)
}

func GetPluginRegistryImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultPluginRegistryImage)
}

func GetDevfileRegistryImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultDevfileRegistryImage)
}

func GetGatewayImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultSingleHostGatewayImage)
}

func GetGatewayConfigSidecarImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultSingleHostGatewayConfigSidecarImage)
}

func GetGatewayAuthenticationSidecarImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultGatewayAuthenticationSidecarImage)
}

func GetGatewayAuthorizationSidecarImage(checluster interface{}) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultGatewayAuthorizationSidecarImage)
}

func GetCheFlavor() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultCheFlavor
}

func GetConsoleLinkName() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultConsoleLinkName
}

func GetConsoleLinkDisplayName() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultConsoleLinkDisplayName
}

func GetConsoleLinkSection() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultConsoleLinkSection
}

func GetConsoleLinkImage() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultsConsoleLinkImage
}

func GetPluginRegistryOpenVSXURL() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultPluginRegistryOpenVSXURL
}

func GetDashboardHeaderMessageText() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultDashboardHeaderMessageText
}

func GetDevEnvironmentsDefaultEditor() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultDevEnvironmentsDefaultEditor
}

func GetDevEnvironmentsDefaultComponents() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultDevEnvironmentsDefaultComponents
}

func GetDevEnvironmentsContainerSecurityContext() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultDevEnvironmentsContainerSecurityContext
}

func GetDevEnvironmentsDisableContainerBuildCapabilities() string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return defaultDevEnvironmentsDisableContainerBuildCapabilities
}

func PatchDefaultImageName(checluster interface{}, imageName string) string {
	checlusterUnstructured, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(checluster)
	hostname, _, _ := unstructured.NestedString(checlusterUnstructured, "spec", "containerRegistry", "hostname")
	organization, _, _ := unstructured.NestedString(checlusterUnstructured, "spec", "containerRegistry", "organization")

	if hostname == "" && organization == "" {
		return imageName
	}

	if hostname == "" {
		hostname = getHostnameFromImage(imageName)
	}

	if organization == "" {
		organization = getOrganizationFromImage(imageName)
	}

	image := GetImageNameFromFullImage(imageName)
	return fmt.Sprintf("%s/%s/%s", hostname, organization, image)
}

func GetImageNameFromFullImage(image string) string {
	imageParts := strings.Split(image, "/")
	nameAndTag := ""
	switch len(imageParts) {
	case 1:
		nameAndTag = imageParts[0]
	case 2:
		nameAndTag = imageParts[1]
	case 3:
		nameAndTag = imageParts[2]
	}
	return nameAndTag
}

func getHostnameFromImage(image string) string {
	imageParts := strings.Split(image, "/")
	hostname := ""
	switch len(imageParts) {
	case 3:
		hostname = imageParts[0]
	default:
		hostname = "docker.io"
	}
	return hostname
}

func getOrganizationFromImage(image string) string {
	imageParts := strings.Split(image, "/")
	organization := ""
	switch len(imageParts) {
	case 2:
		organization = imageParts[0]
	case 3:
		organization = imageParts[1]
	}
	return organization
}
