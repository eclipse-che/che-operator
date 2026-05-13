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

	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	util "github.com/eclipse-che/che-operator/pkg/common/utils"
	appsv1 "k8s.io/api/apps/v1"
)

var (
	defaultCheVersion                                       string
	defaultCheFlavor                                        string
	defaultCheServerImage                                   string
	defaultDashboardImage                                   string
	defaultPluginRegistryImage                              string
	defaultCheTLSSecretsCreationJobImage                    string
	defaultSingleHostGatewayImage                           string
	defaultSingleHostGatewayConfigSidecarImage              string
	defaultGatewayKubernetesAuthenticationSidecarImage      string
	defaultGatewayKubernetesAuthorizationSidecarImage       string
	defaultGatewayOpenShiftAuthenticationSidecarImage       string
	defaultGatewayOpenShiftAuthorizationSidecarImage        string
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
	defaultDevfileRegistryExternalDevfileRegistries         string

	initialized = false

	log = ctrl.Log.WithName("defaults")
)

func InitializeForTesting(operatorDeploymentFilePath string) {
	operatorDeployment := &appsv1.Deployment{}
	if err := util.ReadObjectInto(operatorDeploymentFilePath, operatorDeployment); err != nil {
		log.Error(err, "Error reading operator deployment")
		os.Exit(1)
	}

	for _, container := range operatorDeployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			err := os.Setenv(env.Name, env.Value)
			if err != nil {
				panic(fmt.Sprintf("Error setting env var %s=%s", env.Name, env.Value))
			}
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
	defaultDevfileRegistryExternalDevfileRegistries = os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES")

	defaultCheServerImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_che_server"))
	defaultDashboardImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_dashboard"))
	defaultPluginRegistryImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_plugin_registry"))
	defaultSingleHostGatewayImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_single_host_gateway_config_sidecar"))

	defaultGatewayOpenShiftAuthenticationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authentication_sidecar"))
	defaultGatewayOpenShiftAuthorizationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authorization_sidecar"))
	defaultGatewayKubernetesAuthenticationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authentication_sidecar_k8s"))
	defaultGatewayKubernetesAuthorizationSidecarImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_gateway_authorization_sidecar_k8s"))

	// Don't get some k8s specific env
	if !infrastructure.IsOpenShift() {
		defaultCheTLSSecretsCreationJobImage = ensureEnv(util.GetArchitectureDependentEnvName("RELATED_IMAGE_che_tls_secrets_creation_job"))
	}

	initialized = true
}

func ensureEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Error(fmt.Errorf("environment variable %s not set", name), "unable to determine required environment variable")
		os.Exit(1)
	}

	return value
}

func GetCheServerImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultCheServerImage)
}

func GetCheTLSSecretsCreationJobImage() string {
	if !initialized {
		Initialize()
	}

	return defaultCheTLSSecretsCreationJobImage
}

func GetCheVersion() string {
	if !initialized {
		Initialize()
	}

	return defaultCheVersion
}

func GetDashboardImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultDashboardImage)
}

func GetPluginRegistryImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultPluginRegistryImage)
}

func GetGatewayImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultSingleHostGatewayImage)
}

func GetGatewayConfigSidecarImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultSingleHostGatewayConfigSidecarImage)
}

func GetGatewayKubernetesAuthenticationSidecarImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultGatewayKubernetesAuthenticationSidecarImage)
}

func GetGatewayKubernetesAuthorizationSidecarImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultGatewayKubernetesAuthorizationSidecarImage)
}

func GetGatewayOpenShiftAuthenticationSidecarImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultGatewayOpenShiftAuthenticationSidecarImage)
}

func GetGatewayOpenShiftAuthorizationSidecarImage(checluster interface{}) string {
	if !initialized {
		Initialize()
	}

	return PatchDefaultImageName(checluster, defaultGatewayOpenShiftAuthorizationSidecarImage)
}

func GetCheFlavor() string {
	if !initialized {
		Initialize()
	}

	return defaultCheFlavor
}

func GetConsoleLinkName() string {
	if !initialized {
		Initialize()
	}

	return defaultConsoleLinkName
}

func GetConsoleLinkDisplayName() string {
	if !initialized {
		Initialize()
	}

	return defaultConsoleLinkDisplayName
}

func GetConsoleLinkSection() string {
	if !initialized {
		Initialize()
	}

	return defaultConsoleLinkSection
}

func GetConsoleLinkImage() string {
	if !initialized {
		Initialize()
	}

	return defaultsConsoleLinkImage
}

func GetDevfileRegistryExternalDevfileRegistries() string {
	if !initialized {
		Initialize()
	}

	return defaultDevfileRegistryExternalDevfileRegistries
}

func GetPluginRegistryOpenVSXURL() string {
	if !initialized {
		Initialize()
	}

	return defaultPluginRegistryOpenVSXURL
}

func GetDashboardHeaderMessageText() string {
	if !initialized {
		Initialize()
	}

	return defaultDashboardHeaderMessageText
}

func GetDevEnvironmentsDefaultEditor() string {
	if !initialized {
		Initialize()
	}

	return defaultDevEnvironmentsDefaultEditor
}

func GetDevEnvironmentsDefaultComponents() string {
	if !initialized {
		Initialize()
	}

	return defaultDevEnvironmentsDefaultComponents
}

func GetDevEnvironmentsContainerSecurityContext() string {
	if !initialized {
		Initialize()
	}

	return defaultDevEnvironmentsContainerSecurityContext
}

func GetDevEnvironmentsDisableContainerBuildCapabilities() string {
	if !initialized {
		Initialize()
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
