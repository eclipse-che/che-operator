//
// Copyright (c) 2018-2020 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
// REMINDER: when updating versions below, see also pkg/apis/org/v1/che_types.go and deploy/crds/org_v1_che_cr.yaml
package deploy

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	util "github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

var (
	defaultCheServerImage       string
	defaultCheVersion           string
	defaultPluginRegistryImage  string
	defaultDevfileRegistryImage string
	defaultPvcJobsImage         string
	defaultPostgresImage        string
	defaultKeycloakImage        string

	defaultCheWorkspacePluginBrokerMetadataImage  string
	defaultCheWorkspacePluginBrokerArtifactsImage string
	defaultCheServerSecureExposerJwtProxyImage    string
)

const (
	DefaultCheFlavor           = "che"
	DefaultChePostgresUser     = "pgche"
	DefaultChePostgresHostName = "postgres"
	DefaultChePostgresPort     = "5432"
	DefaultChePostgresDb       = "dbche"
	DefaultPvcStrategy         = "common"
	DefaultPvcClaimSize        = "1Gi"
	DefaultIngressStrategy     = "multi-host"
	DefaultIngressClass        = "nginx"

	DefaultPluginRegistryMemoryLimit   = "256Mi"
	DefaultPluginRegistryMemoryRequest = "16Mi"

	DefaultDevfileRegistryMemoryLimit   = "256Mi"
	DefaultDevfileRegistryMemoryRequest = "16Mi"
	DefaultKeycloakAdminUserName        = "admin"
	DefaultCheLogLevel                  = "INFO"
	DefaultCheDebug                     = "false"
	DefaultCheMultiUser                 = "true"
	DefaultCheMetricsPort               = int32(8087)
	DefaultCheDebugPort                 = int32(8000)
	DefaultCheVolumeMountPath           = "/data"
	DefaultCheVolumeClaimName           = "che-data-volume"
	DefaultPostgresVolumeClaimName      = "postgres-data"

	DefaultJavaOpts = "-XX:MaxRAMFraction=2 -XX:+UseParallelGC -XX:MinHeapFreeRatio=10 " +
		"-XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 " +
		"-XX:AdaptiveSizePolicyWeight=90 -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap " +
		"-Dsun.zip.disableMemoryMapping=true -Xms20m"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
	DefaultServerMemoryRequest      = "512Mi"
	DefaultServerMemoryLimit        = "1Gi"
	DefaultSecurityContextFsGroup   = "1724"
	DefaultSecurityContextRunAsUser = "1724"

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next version
	// That fixed bug https://github.com/eclipse/che/issues/13714
	OldDefaultKeycloakUpstreamImageToDetect = "eclipse/che-keycloak:7.0.0"
	OldDefaultPvcJobsUpstreamImageToDetect  = "registry.access.redhat.com/ubi8-minimal:8.0-127"
	OldDefaultPostgresUpstreamImageToDetect = "centos/postgresql-96-centos7:9.6"

	OldDefaultCodeReadyServerImageRepo = "registry.redhat.io/codeready-workspaces/server-rhel8"
	OldDefaultCodeReadyServerImageTag  = "1.2"
	OldCrwPluginRegistryUrl            = "https://che-plugin-registry.openshift.io"

	// ConsoleLink default
	DefaultConsoleLinkName                = "che"
	DefaultConsoleLinkSection             = "Red Hat Applications"
	DefaultConsoleLinkImage               = "/dashboard/assets/branding/loader.svg"
	defaultConsoleLinkUpstreamDisplayName = "Eclipse Che"
	defaultConsoleLinkDisplayName         = "CodeReady Workspaces"
)

func InitDefaults(defaultsPath string) {
	if defaultsPath == "" {
		InitDefaultsFromEnv()
	} else {
		InitDefaultsFromFile(defaultsPath)
	}
}

func InitDefaultsFromEnv() {
	defaultCheVersion = getDefaultFromEnv("CHE_VERSION")
	defaultCheServerImage = getDefaultFromEnv("IMAGE_default_che_server")
	defaultPluginRegistryImage = getDefaultFromEnv("IMAGE_default_plugin_registry")
	defaultDevfileRegistryImage = getDefaultFromEnv("IMAGE_default_devfile_registry")
	defaultPvcJobsImage = getDefaultFromEnv("IMAGE_default_pvc_jobs")
	defaultPostgresImage = getDefaultFromEnv("IMAGE_default_postgres")
	defaultKeycloakImage = getDefaultFromEnv("IMAGE_default_keycloak")

	// CRW images for that are mentioned in the Che server che.properties
	// For CRW these should be synced by hand with images stored in RH registries
	// instead of being synced by script with the content of the upstream `che.properties` file
	defaultCheWorkspacePluginBrokerMetadataImage = getDefaultFromEnv("IMAGE_default_che_workspace_plugin_broker_metadata")
	defaultCheWorkspacePluginBrokerArtifactsImage = getDefaultFromEnv("IMAGE_default_che_workspace_plugin_broker_artifacts")
	defaultCheServerSecureExposerJwtProxyImage = getDefaultFromEnv("IMAGE_default_che_server_secure_exposer_jwt_proxy_image")
}

func InitDefaultsFromFile(defaultsPath string) {
	operatorDeployment := getDefaultsFromFile(defaultsPath)

	defaultCheVersion = util.GetDeploymentEnv(operatorDeployment, "CHE_VERSION")
	defaultCheServerImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_che_server")
	defaultPluginRegistryImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_plugin_registry")
	defaultDevfileRegistryImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_devfile_registry")
	defaultPvcJobsImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_pvc_jobs")
	defaultPostgresImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_postgres")
	defaultKeycloakImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_keycloak")
	defaultCheWorkspacePluginBrokerMetadataImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_che_workspace_plugin_broker_metadata")
	defaultCheWorkspacePluginBrokerArtifactsImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_che_workspace_plugin_broker_artifacts")
	defaultCheServerSecureExposerJwtProxyImage = util.GetDeploymentEnv(operatorDeployment, "IMAGE_default_che_server_secure_exposer_jwt_proxy_image")
}

func getDefaultsFromFile(defaultsPath string) *v1.Deployment {
	bytes, err := ioutil.ReadFile(defaultsPath)
	if err != nil {
		logrus.Fatalf("Unable to read file with defaults by path %s", defaultsPath)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(bytes, nil, nil)
	if err != nil {
		logrus.Fatalf(fmt.Sprintf("Error while decoding YAML object with defaults. Err was: %s", err))
	}

	deployment, ok := obj.(*v1.Deployment)
	if ok {
		return deployment
	}
	logrus.Fatalf("File %s doesn't contains real deployment.", defaultsPath)
	return nil
}

func getDefaultFromEnv(envName string) string {
	value := os.Getenv(envName)

	if len(value) == 0 {
		logrus.Fatalf("Failed to initialize default value: '%s'. Environment variable with default value was not found.", envName)
	}

	return value
}

func MigratingToCRW2_0(cr *orgv1.CheCluster) bool {
	if cr.Spec.Server.CheFlavor == "codeready" &&
		strings.HasPrefix(cr.Status.CheVersion, "1.2") &&
		strings.HasPrefix(defaultCheVersion, "2.0") {
		return true
	}
	return false
}

func DefaultConsoleLinkDisplayName(cheFlavor string) string {
	if cheFlavor == "codeready" {
		return defaultConsoleLinkDisplayName
	}
	return defaultConsoleLinkUpstreamDisplayName
}

func DefaultCheVersion() string {
	return defaultCheVersion
}

func DefaultCheServerImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultCheServerImage)
}

func DefaultPvcJobsImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultPvcJobsImage)
}

func DefaultPostgresImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultPostgresImage)
}

func DefaultKeycloakImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultKeycloakImage)
}

func DefaultPluginRegistryImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultPluginRegistryImage)
}

func DefaultDevfileRegistryImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultDevfileRegistryImage)
}

func DefaultCheWorkspacePluginBrokerMetadataImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerMetadataImage)
}

func DefaultCheWorkspacePluginBrokerArtifactsImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultCheWorkspacePluginBrokerArtifactsImage)
}

func DefaultCheServerSecureExposerJwtProxyImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultCheServerSecureExposerJwtProxyImage)
}

func DefaultPullPolicyFromDockerImage(dockerImage string) string {
	tag := "latest"
	parts := strings.Split(dockerImage, ":")
	if len(parts) > 1 {
		tag = parts[1]
	}
	if tag == "latest" || tag == "nightly" {
		return "Always"
	}
	return "IfNotPresent"
}

func GetCheMultiUser(cr *orgv1.CheCluster) string {
	if cr.Spec.Server.CustomCheProperties != nil {
		cheMultiUser := cr.Spec.Server.CustomCheProperties["CHE_MULTIUSER"]
		if cheMultiUser == "false" {
			return "false"
		}
	}
	return DefaultCheMultiUser
}

func patchDefaultImageName(cr *orgv1.CheCluster, imageName string) string {
	if !cr.IsAirGapMode() {
		return imageName
	}
	var hostname, organization string
	if cr.Spec.Server.AirGapContainerRegistryHostname != "" {
		hostname = cr.Spec.Server.AirGapContainerRegistryHostname
	} else {
		hostname = getHostnameFromImage(imageName)
	}
	if cr.Spec.Server.AirGapContainerRegistryOrganization != "" {
		organization = cr.Spec.Server.AirGapContainerRegistryOrganization
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
