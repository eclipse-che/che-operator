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
package deploy

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
)

var (
	defaultCheServerImage                      string
	defaultCheVersion                          string
	defaultDashboardImage                      string
	defaultDevworkspaceControllerImage         string
	defaultPluginRegistryImage                 string
	defaultDevfileRegistryImage                string
	defaultCheTLSSecretsCreationJobImage       string
	defaultPvcJobsImage                        string
	defaultPostgresImage                       string
	defaultPostgres13Image                     string
	defaultSingleHostGatewayImage              string
	defaultSingleHostGatewayConfigSidecarImage string
	defaultGatewayAuthenticationSidecarImage   string
	defaultGatewayAuthorizationSidecarImage    string
	defaultGatewayHeaderProxySidecarImage      string

	defaultCheWorkspacePluginBrokerMetadataImage  string
	defaultCheWorkspacePluginBrokerArtifactsImage string
	defaultCheServerSecureExposerJwtProxyImage    string
	DefaultSingleHostGatewayConfigMapLabels       = map[string]string{
		"app":       "che",
		"component": "che-gateway-config",
	}
)

const (
	DefaultChePostgresUser              = "pgche"
	DefaultChePostgresHostName          = "postgres"
	DefaultChePostgresPort              = "5432"
	DefaultChePostgresDb                = "dbche"
	DefaultPvcStrategy                  = "common"
	DefaultPvcClaimSize                 = "10Gi"
	DefaultIngressClass                 = "nginx"
	DefaultChePostgresCredentialsSecret = "postgres-credentials"

	DefaultCheLogLevel             = "INFO"
	DefaultCheDebug                = "false"
	DefaultCheMetricsPort          = int32(8087)
	DefaultCheDebugPort            = int32(8000)
	DefaultPostgresVolumeClaimName = "postgres-data"
	DefaultPostgresPvcClaimSize    = "1Gi"

	DefaultJavaOpts          = "-XX:MaxRAMPercentage=85.0"
	DefaultWorkspaceJavaOpts = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"
	DefaultSecurityContextFsGroup   = "1724"
	DefaultSecurityContextRunAsUser = "1724"

	KubernetesImagePullerOperatorCSV = "kubernetes-imagepuller-operator.v0.0.9"

	ServerExposureStrategy        = "single-host"
	GatewaySingleHostExposureType = "gateway"

	// kubernetes default labels
	KubernetesComponentLabelKey = "app.kubernetes.io/component"
	KubernetesPartOfLabelKey    = "app.kubernetes.io/part-of"
	KubernetesManagedByLabelKey = "app.kubernetes.io/managed-by"
	KubernetesInstanceLabelKey  = "app.kubernetes.io/instance"
	KubernetesNameLabelKey      = "app.kubernetes.io/name"

	CheEclipseOrg         = "che.eclipse.org"
	OAuthScmConfiguration = "oauth-scm-configuration"

	// che.eclipse.org annotations
	CheEclipseOrgMountPath                = "che.eclipse.org/mount-path"
	CheEclipseOrgMountAs                  = "che.eclipse.org/mount-as"
	CheEclipseOrgEnvName                  = "che.eclipse.org/env-name"
	CheEclipseOrgNamespace                = "che.eclipse.org/namespace"
	CheEclipseOrgGithubOAuthCredentials   = "che.eclipse.org/github-oauth-credentials"
	CheEclipseOrgOAuthScmServer           = "che.eclipse.org/oauth-scm-server"
	CheEclipseOrgScmServerEndpoint        = "che.eclipse.org/scm-server-endpoint"
	CheEclipseOrgHash256                  = "che.eclipse.org/hash256"
	CheEclipseOrgManagedAnnotationsDigest = "che.eclipse.org/managed-annotations-digest"

	// components
	DevfileRegistryName = "devfile-registry"
	PluginRegistryName  = "plugin-registry"
	PostgresName        = "postgres"

	// CheServiceAccountName - service account name for che-server.
	CheServiceAccountName = "che"

	// Name of the secret that holds self-signed certificate for git connections
	GitSelfSignedCertsConfigMapName = "che-git-self-signed-cert"

	CheTLSSelfSignedCertificateSecretName = "self-signed-certificate"
	DefaultCheTLSSecretName               = "che-tls"

	// limits
	DefaultDashboardMemoryLimit   = "256Mi"
	DefaultDashboardMemoryRequest = "32Mi"
	DefaultDashboardCpuLimit      = "500m"
	DefaultDashboardCpuRequest    = "100m"

	DefaultPluginRegistryMemoryLimit   = "256Mi"
	DefaultPluginRegistryMemoryRequest = "32Mi"
	DefaultPluginRegistryCpuLimit      = "500m"
	DefaultPluginRegistryCpuRequest    = "100m"

	DefaultDevfileRegistryMemoryLimit   = "256Mi"
	DefaultDevfileRegistryMemoryRequest = "32Mi"
	DefaultDevfileRegistryCpuLimit      = "500m"
	DefaultDevfileRegistryCpuRequest    = "100m"

	DefaultServerMemoryLimit   = "1024Mi"
	DefaultServerMemoryRequest = "512Mi"
	DefaultServerCpuLimit      = "1"
	DefaultServerCpuRequest    = "100m"

	DefaultIdentityProviderMemoryLimit   = "1536Mi"
	DefaultIdentityProviderMemoryRequest = "1024Mi"
	DefaultIdentityProviderCpuLimit      = "2"
	DefaultIdentityProviderCpuRequest    = "100m"

	DefaultPostgresMemoryLimit   = "1024Mi"
	DefaultPostgresMemoryRequest = "512Mi"
	DefaultPostgresCpuLimit      = "500m"
	DefaultPostgresCpuRequest    = "100m"

	BitBucketOAuthConfigMountPath           = "/che-conf/oauth/bitbucket"
	BitBucketOAuthConfigPrivateKeyFileName  = "private.key"
	BitBucketOAuthConfigConsumerKeyFileName = "consumer.key"

	GitHubOAuthConfigMountPath            = "/che-conf/oauth/github"
	GitHubOAuthConfigClientIdFileName     = "id"
	GitHubOAuthConfigClientSecretFileName = "secret"

	GitLabSaasOAuthConfigMountPath            = "/che-conf/oauth/gitlab-saas"
	GitLabSaasOAuthConfigClientIdFileName     = "id"
	GitLabSaasOAuthConfigClientSecretFileName = "secret"

	GitLabOAuthConfigMountPath            = "/che-conf/oauth/gitlab"
	GitLabOAuthConfigClientIdFileName     = "id"
	GitLabOAuthConfigClientSecretFileName = "secret"

	InstallOrUpdateFailed                = "InstallOrUpdateFailed"
	DefaultServerTrustStoreConfigMapName = "ca-certs"
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
	defaultDashboardImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_dashboard"))
	defaultDevworkspaceControllerImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_devworkspace_controller"))
	defaultPluginRegistryImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	defaultPvcJobsImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_pvc_jobs"))
	defaultPostgresImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))
	defaultPostgres13Image = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres_13_3"))
	defaultSingleHostGatewayImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway_config_sidecar"))
	defaultGatewayAuthenticationSidecarImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar"))
	defaultGatewayAuthorizationSidecarImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar"))

	// Don't get some k8s specific env
	if !util.IsOpenShift {
		defaultCheTLSSecretsCreationJobImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_che_tls_secrets_creation_job"))
		defaultGatewayAuthenticationSidecarImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar_k8s"))
		defaultGatewayAuthorizationSidecarImage = util.GetDeploymentEnv(operatorDeployment, util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar_k8s"))
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

func IsComponentReadinessInitContainersConfigured(cr *orgv1.CheCluster) bool {
	return os.Getenv("ADD_COMPONENT_READINESS_INIT_CONTAINERS") == "true"
}

func DefaultCheFlavor(cr *orgv1.CheCluster) string {
	return getDefaultFromEnv("CHE_FLAVOR")
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

func DefaultCheVersion() string {
	return defaultCheVersion
}

func DefaultCheServerImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultCheServerImage)
}

func DefaultCheTLSSecretsCreationJobImage() string {
	return defaultCheTLSSecretsCreationJobImage
}

func DefaultPvcJobsImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultPvcJobsImage)
}

func DefaultPostgresImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultPostgresImage)
}

func DefaultPostgres13Image(cr *orgv1.CheCluster) string {
	// it might be empty value until it propertly downstreamed
	if defaultPostgres13Image == "" {
		return defaultPostgres13Image
	}
	return PatchDefaultImageName(cr, defaultPostgres13Image)
}

func DefaultDashboardImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultDashboardImage)
}

func DefaultDevworkspaceControllerImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultDevworkspaceControllerImage)
}

func DefaultPluginRegistryImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultPluginRegistryImage)
}

func DefaultDevfileRegistryImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultDevfileRegistryImage)
}

func DefaultCheWorkspacePluginBrokerMetadataImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultCheWorkspacePluginBrokerMetadataImage)
}

func DefaultCheWorkspacePluginBrokerArtifactsImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultCheWorkspacePluginBrokerArtifactsImage)
}

func DefaultCheServerSecureExposerJwtProxyImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultCheServerSecureExposerJwtProxyImage)
}

func DefaultSingleHostGatewayImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultSingleHostGatewayImage)
}

func DefaultSingleHostGatewayConfigSidecarImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultSingleHostGatewayConfigSidecarImage)
}

func DefaultGatewayAuthenticationSidecarImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultGatewayAuthenticationSidecarImage)
}

func DefaultGatewayAuthorizationSidecarImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultGatewayAuthorizationSidecarImage)
}

func DefaultGatewayHeaderProxySidecarImage(cr *orgv1.CheCluster) string {
	return PatchDefaultImageName(cr, defaultGatewayHeaderProxySidecarImage)
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
	if tag == "latest" || tag == "nightly" || tag == "next" {
		return "Always"
	}
	return "IfNotPresent"
}

// GetWorkspaceNamespaceDefault - returns workspace namespace default strategy, which points on the namespaces used for workspaces execution.
func GetWorkspaceNamespaceDefault(cr *orgv1.CheCluster) string {
	if cr.Spec.Server.CustomCheProperties != nil {
		k8sNamespaceDefault := cr.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"]
		if k8sNamespaceDefault != "" {
			return k8sNamespaceDefault
		}
	}

	workspaceNamespaceDefault := cr.Namespace
	if util.IsOpenShift {
		workspaceNamespaceDefault = "<username>-" + DefaultCheFlavor(cr)
	}
	return util.GetValue(cr.Spec.Server.WorkspaceNamespaceDefault, workspaceNamespaceDefault)
}

func PatchDefaultImageName(cr *orgv1.CheCluster, imageName string) string {
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
	defaultDashboardImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_dashboard"))
	defaultDevworkspaceControllerImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_devworkspace_controller"))
	defaultPluginRegistryImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_plugin_registry"))
	defaultDevfileRegistryImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_devfile_registry"))
	defaultPvcJobsImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_pvc_jobs"))
	defaultPostgresImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres"))

	// allow not to set env variable into a container
	// while downstream is not migrated to PostgreSQL 13.3 yet
	defaultPostgres13Image = os.Getenv(util.GetArchitectureDependentEnv("RELATED_IMAGE_postgres_13_3"))

	defaultSingleHostGatewayImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway"))
	defaultSingleHostGatewayConfigSidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_single_host_gateway_config_sidecar"))
	defaultGatewayAuthenticationSidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar"))
	defaultGatewayAuthorizationSidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar"))
	defaultGatewayHeaderProxySidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_header_sidecar"))

	// Don't get some k8s specific env
	if !util.IsOpenShift {
		defaultCheTLSSecretsCreationJobImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_che_tls_secrets_creation_job"))
		defaultGatewayAuthenticationSidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authentication_sidecar_k8s"))
		defaultGatewayAuthorizationSidecarImage = getDefaultFromEnv(util.GetArchitectureDependentEnv("RELATED_IMAGE_gateway_authorization_sidecar_k8s"))
	}
}

func InitTestDefaultsFromDeployment(deploymentFile string) error {
	operator := &appsv1.Deployment{}
	err := util.ReadObject(deploymentFile, operator)
	if err != nil {
		return err
	}

	for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
		err = os.Setenv(env.Name, env.Value)
		if err != nil {
			return err
		}
	}

	os.Setenv("MOCK_API", "1")
	InitDefaultsFromEnv()
	return nil
}
