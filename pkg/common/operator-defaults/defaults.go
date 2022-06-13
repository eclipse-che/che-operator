//
// Copyright (c) 2019-2021 Red Hat, Inc.
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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	util "github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
)

var (
	defaultCheVersion                             string
	defaultCheFlavor                              string
	defaultCheServerImage                         string
	defaultDashboardImage                         string
	defaultDevworkspaceControllerImage            string
	defaultPluginRegistryImage                    string
	defaultDevfileRegistryImage                   string
	defaultCheTLSSecretsCreationJobImage          string
	defaultPostgresImage                          string
	defaultPostgres13Image                        string
	defaultSingleHostGatewayImage                 string
	defaultSingleHostGatewayConfigSidecarImage    string
	defaultGatewayAuthenticationSidecarImage      string
	defaultGatewayAuthorizationSidecarImage       string
	defaultCheWorkspacePluginBrokerMetadataImage  string
	defaultCheWorkspacePluginBrokerArtifactsImage string
	defaultCheServerSecureExposerJwtProxyImage    string
	defaultConsoleLinkName                        string
	defaultConsoleLinkDisplayName                 string
	defaultConsoleLinkSection                     string
	defaultsConsoleLinkImage                      string

	initialized = false
)

func Initialize(operatorDeploymentFilePath string) {
	if operatorDeploymentFilePath == "" {
		InitializeFromEnv()
	} else {
		InitializeFromFile(operatorDeploymentFilePath)
	}

}

func InitializeFromFile(operatorDeploymentFilePath string) {
	operatorDeployment := &appsv1.Deployment{}
	if err := util.ReadObjectInto(operatorDeploymentFilePath, operatorDeployment); err != nil {
		logrus.Fatalf("Failed to read operator deployment from '%s', cause: %v", operatorDeploymentFilePath, err)
	}

	containers := operatorDeployment.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		logrus.Fatalf("Containers not found in operator deployment '%s'", operatorDeploymentFilePath)
	}

	defaultCheVersion = util.GetEnv(containers[0].Env, "CHE_VERSION")
	defaultCheFlavor = util.GetEnv(containers[0].Env, "CHE_FLAVOR")
	defaultConsoleLinkDisplayName = util.GetEnv(containers[0].Env, "CONSOLE_LINK_DISPLAY_NAME")
	defaultConsoleLinkName = util.GetEnv(containers[0].Env, "CONSOLE_LINK_NAME")
	defaultConsoleLinkSection = util.GetEnv(containers[0].Env, "CONSOLE_LINK_SECTION")
	defaultsConsoleLinkImage = util.GetEnv(containers[0].Env, "CONSOLE_LINK_IMAGE")

	defaultCheServerImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server"))
	defaultDashboardImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_dashboard"))
	defaultDevworkspaceControllerImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_devworkspace_controller"))
	defaultPluginRegistryImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	defaultPostgresImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))
	defaultPostgres13Image = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres_13_3"))
	defaultSingleHostGatewayImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway_config_sidecar"))
	defaultGatewayAuthenticationSidecarImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar"))
	defaultGatewayAuthorizationSidecarImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar"))

	// Don't get some k8s specific env
	if !infrastructure.IsOpenShift() {
		defaultCheTLSSecretsCreationJobImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_tls_secrets_creation_job"))
		defaultGatewayAuthenticationSidecarImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar_k8s"))
		defaultGatewayAuthorizationSidecarImage = util.GetEnv(containers[0].Env, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar_k8s"))
	}

	initialized = true
}

func InitializeFromEnv() {
	defaultCheVersion = ensureEnv("CHE_VERSION")
	defaultCheFlavor = ensureEnv("CHE_FLAVOR")
	defaultConsoleLinkDisplayName = ensureEnv("CONSOLE_LINK_DISPLAY_NAME")
	defaultConsoleLinkName = ensureEnv("CONSOLE_LINK_NAME")
	defaultConsoleLinkSection = ensureEnv("CONSOLE_LINK_SECTION")
	defaultsConsoleLinkImage = ensureEnv("CONSOLE_LINK_IMAGE")

	defaultCheServerImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server"))
	defaultDashboardImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_dashboard"))
	defaultDevworkspaceControllerImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_devworkspace_controller"))
	defaultPluginRegistryImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	defaultPostgresImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))

	// allow not to set env variable into a container
	// while downstream is not migrated to PostgreSQL 13.3 yet
	defaultPostgres13Image = os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres_13_3"))

	defaultSingleHostGatewayImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway_config_sidecar"))
	defaultGatewayAuthenticationSidecarImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar"))
	defaultGatewayAuthorizationSidecarImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar"))

	// Don't get some k8s specific env
	if !infrastructure.IsOpenShift() {
		defaultCheTLSSecretsCreationJobImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_tls_secrets_creation_job"))
		defaultGatewayAuthenticationSidecarImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar_k8s"))
		defaultGatewayAuthorizationSidecarImage = ensureEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar_k8s"))
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

func GetCheServerImage(checluster *chev2.CheCluster) string {
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

func GetPostgresImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultPostgresImage)
}

func GetPostgres13Image(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultPostgres13Image)
}

func GetDashboardImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultDashboardImage)
}

func GetDevworkspaceControllerImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultDevworkspaceControllerImage)
}

func GetPluginRegistryImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultPluginRegistryImage)
}

func GetDevfileRegistryImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultDevfileRegistryImage)
}

func GetCheWorkspacePluginBrokerMetadataImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultCheWorkspacePluginBrokerMetadataImage)
}

func GetCheWorkspacePluginBrokerArtifactsImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultCheWorkspacePluginBrokerArtifactsImage)
}

func GetCheServerSecureExposerJwtProxyImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultCheServerSecureExposerJwtProxyImage)
}

func GetGatewayImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultSingleHostGatewayImage)
}

func GetGatewayConfigSidecarImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultSingleHostGatewayConfigSidecarImage)
}

func GetGatewayAuthenticationSidecarImage(checluster *chev2.CheCluster) string {
	if !initialized {
		logrus.Fatalf("Operator defaults are not initialized.")
	}

	return PatchDefaultImageName(checluster, defaultGatewayAuthenticationSidecarImage)
}

func GetGatewayAuthorizationSidecarImage(checluster *chev2.CheCluster) string {
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

func IsComponentReadinessInitContainersConfigured() bool {
	return os.Getenv("ADD_COMPONENT_READINESS_INIT_CONTAINERS") == "true"
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

func PatchDefaultImageName(checluster *chev2.CheCluster, imageName string) string {
	if !checluster.IsAirGapMode() {
		return imageName
	}

	var hostname, organization string
	if checluster.Spec.ContainerRegistry.Hostname != "" {
		hostname = checluster.Spec.ContainerRegistry.Hostname
	} else {
		hostname = getHostnameFromImage(imageName)
	}

	if checluster.Spec.ContainerRegistry.Organization != "" {
		organization = checluster.Spec.ContainerRegistry.Organization
	} else {
		organization = getOrganizationFromImage(imageName)
	}

	image := getImageNameFromFullImage(imageName)
	return fmt.Sprintf("%s/%s/%s", hostname, organization, image)
}

func getImageNameFromFullImage(image string) string {
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
