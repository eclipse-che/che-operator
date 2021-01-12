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

	"gopkg.in/yaml.v2"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
)

var (
	defaultCheServerImage                      string
	defaultCheVersion                          string
	defaultPluginRegistryImage                 string
	defaultDevfileRegistryImage                string
	defaultCheTLSSecretsCreationJobImage       string
	defaultPvcJobsImage                        string
	defaultPostgresImage                       string
	defaultKeycloakImage                       string
	defaultSingleHostGatewayImage              string
	defaultSingleHostGatewayConfigSidecarImage string

	defaultCheWorkspacePluginBrokerMetadataImage  string
	defaultCheWorkspacePluginBrokerArtifactsImage string
	defaultCheServerSecureExposerJwtProxyImage    string
	DefaultSingleHostGatewayConfigMapLabels       = map[string]string{
		"app":       "che",
		"component": "che-gateway-config",
	}
)

const (
	DefaultChePostgresUser     = "pgche"
	DefaultChePostgresHostName = "postgres"
	DefaultChePostgresPort     = "5432"
	DefaultChePostgresDb       = "dbche"
	DefaultPvcStrategy         = "common"
	DefaultPvcClaimSize        = "1Gi"
	DefaultIngressClass        = "nginx"

	DefaultPluginRegistryMemoryLimit   = "256Mi"
	DefaultPluginRegistryMemoryRequest = "16Mi"

	// DefaultKube
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

	DefaultJavaOpts          = "-XX:MaxRAMPercentage=85.0"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
	DefaultServerMemoryRequest      = "512Mi"
	DefaultServerMemoryLimit        = "1Gi"
	DefaultSecurityContextFsGroup   = "1724"
	DefaultSecurityContextRunAsUser = "1724"

	KubernetesImagePullerOperatorCSV = "kubernetes-imagepuller-operator.v0.0.4"

	DefaultServerExposureStrategy           = "multi-host"
	DefaultKubernetesSingleHostExposureType = "native"
	DefaultOpenShiftSingleHostExposureType  = "gateway"

	// This is only to correctly  manage defaults during the transition
	// from Upstream 7.0.0 GA to the next version
	// That fixed bug https://github.com/eclipse/che/issues/13714
	OldDefaultKeycloakUpstreamImageToDetect = "eclipse/che-keycloak:7.0.0"
	OldDefaultPvcJobsUpstreamImageToDetect  = "registry.access.redhat.com/ubi8-minimal:8.0-127"
	OldDefaultPostgresUpstreamImageToDetect = "centos/postgresql-96-centos7:9.6"

	OldDefaultCodeReadyServerImageRepo = "registry.redhat.io/codeready-workspaces/server-rhel8"
	OldDefaultCodeReadyServerImageTag  = "1.2"
	OldCrwPluginRegistryUrl            = "https://che-plugin-registry.openshift.io"

	// kubernetes default labels
	KubernetesComponentLabelKey = "app.kubernetes.io/component"
	KubernetesPartOfLabelKey    = "app.kubernetes.io/part-of"
	KubernetesManagedByLabelKey = "app.kubernetes.io/managed-by"
	KubernetesInstanceLabelKey  = "app.kubernetes.io/instance"
	KubernetesNameLabelKey      = "app.kubernetes.io/name"

	CheEclipseOrg = "che.eclipse.org"

	// che.eclipse.org annotations
	CheEclipseOrgMountPath = "che.eclipse.org/mount-path"
	CheEclipseOrgMountAs   = "che.eclipse.org/mount-as"
	CheEclipseOrgEnvName   = "che.eclipse.org/env-name"
	CheEclipseOrgNamespace = "che.eclipse.org/namespace"

	// components
	IdentityProviderName = "keycloak"
	DevfileRegistryName  = "devfile-registry"
	PluginRegistryName   = "plugin-registry"
	PostgresName         = "postgres"
)

func InitDefaults(defaultsPath string) {
	if defaultsPath == "" {
		InitDefaultsFromEnv()
	} else {
		InitDefaultsFromFile(defaultsPath)
	}
}

func InitDefaultsFromFile(defaultsPath string) {
	operatorDeployment := getDefaultsFromFile(defaultsPath)

	defaultCheVersion = util.GetDeploymentEnv(operatorDeployment, "CHE_VERSION")
	defaultCheServerImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server"))
	defaultPluginRegistryImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	defaultPvcJobsImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_pvc_jobs"))
	defaultPostgresImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))
	defaultKeycloakImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_keycloak"))
	defaultSingleHostGatewayImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway_config_sidecar"))
	defaultCheWorkspacePluginBrokerMetadataImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_workspace_plugin_broker_metadata"))
	defaultCheWorkspacePluginBrokerArtifactsImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts"))
	defaultCheServerSecureExposerJwtProxyImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image"))

	// Don't get some k8s specific env
	if !util.IsOpenShift {
		defaultCheTLSSecretsCreationJobImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_tls_secrets_creation_job"))
	}
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

func DefaultServerTrustStoreConfigMapName() string {
	return getDefaultFromEnv("CHE_SERVER_TRUST_STORE_CONFIGMAP_NAME")
}

func DefaultCheFlavor(cr *orgv1.CheCluster) string {
	return util.GetValue(cr.Spec.Server.CheFlavor, getDefaultFromEnv("CHE_FLAVOR"))
}

func DefaultConsoleLinkName() string {
	return getDefaultFromEnv("CONSOLE_LINK_NAME")
}

func DefaultConsoleLinkDisplayName() string {
	return getDefaultFromEnv("CONSOLE_LINK_DISPLAY_NAME")
}

func DefaultConsoleLinkSection() string {
	return getDefaultFromEnv("CONSOLE_LINK_SECTION")
}

func DefaultConsoleLinkImage() string {
	return getDefaultFromEnv("CONSOLE_LINK_IMAGE")
}

func DefaultCheIdentitySecret() string {
	return getDefaultFromEnv("CHE_IDENTITY_SECRET")
}

func DefaultCheIdentityPostgresSecret() string {
	return getDefaultFromEnv("CHE_IDENTITY_POSTGRES_SECRET")
}

func DefaultChePostgresSecret() string {
	return getDefaultFromEnv("CHE_POSTGRES_SECRET")
}

func DefaultCheVersion() string {
	return defaultCheVersion
}

func DefaultCheServerImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultCheServerImage)
}

func DefaultCheTLSSecretsCreationJobImage() string {
	return defaultCheTLSSecretsCreationJobImage
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

func DefaultSingleHostGatewayImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultSingleHostGatewayImage)
}

func DefaultSingleHostGatewayConfigSidecarImage(cr *orgv1.CheCluster) string {
	return patchDefaultImageName(cr, defaultSingleHostGatewayConfigSidecarImage)
}

func DefaultKubernetesImagePullerOperatorCSV() string {
	return KubernetesImagePullerOperatorCSV
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

func GetSingleHostExposureType(cr *orgv1.CheCluster) string {
	if util.IsOpenShift {
		return DefaultOpenShiftSingleHostExposureType
	}

	return util.GetValue(cr.Spec.K8s.SingleHostExposureType, DefaultKubernetesSingleHostExposureType)
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

func InitDefaultsFromEnv() {
	defaultCheVersion = getDefaultFromEnv("CHE_VERSION")
	defaultCheServerImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server"))
	defaultPluginRegistryImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	defaultPvcJobsImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_pvc_jobs"))
	defaultPostgresImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))
	defaultKeycloakImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_keycloak"))
	defaultSingleHostGatewayImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway_config_sidecar"))

	// CRW images for that are mentioned in the Che server che.properties
	// For CRW these should be synced by hand with images stored in RH registries
	// instead of being synced by script with the content of the upstream `che.properties` file
	defaultCheWorkspacePluginBrokerMetadataImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_workspace_plugin_broker_metadata"))
	defaultCheWorkspacePluginBrokerArtifactsImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts"))
	defaultCheServerSecureExposerJwtProxyImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_server_secure_exposer_jwt_proxy_image"))

	// Don't get some k8s specific env
	if !util.IsOpenShift {
		defaultCheTLSSecretsCreationJobImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_tls_secrets_creation_job"))
	}
}

func InitTestDefaultsFromDeployment(deploymentFile string) error {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile(deploymentFile)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, operator)
	if err != nil {
		return err
	}

	for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
		err = os.Setenv(env.Name, env.Value)
		if err != nil {
			return err
		}
	}

	InitDefaultsFromEnv()
	return nil
}
