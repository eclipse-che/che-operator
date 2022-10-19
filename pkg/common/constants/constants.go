//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package constants

const (
	// PostgresSQL
	DefaultPostgresUser              = "pgche"
	DefaultPostgresHostName          = "postgres"
	DefaultPostgresPort              = "5432"
	DefaultPostgresDb                = "dbche"
	DefaultPostgresMemoryLimit       = "1024Mi"
	DefaultPostgresMemoryRequest     = "512Mi"
	DefaultPostgresCpuLimit          = "500m"
	DefaultPostgresCpuRequest        = "100m"
	DefaultPostgresCredentialsSecret = "postgres-credentials"
	DefaultPostgresVolumeClaimName   = "postgres-data"
	DefaultPostgresPvcClaimSize      = "1Gi"

	// Dashboard
	DefaultDashboardMemoryLimit   = "256Mi"
	DefaultDashboardMemoryRequest = "32Mi"
	DefaultDashboardCpuLimit      = "500m"
	DefaultDashboardCpuRequest    = "100m"

	// PluginRegistry
	DefaultPluginRegistryMemoryLimit                          = "256Mi"
	DefaultPluginRegistryMemoryLimitEmbeddedOpenVSXRegistry   = "4Gi"
	DefaultPluginRegistryMemoryRequest                        = "32Mi"
	DefaultPluginRegistryMemoryRequestEmbeddedOpenVSXRegistry = "512Mi"
	DefaultPluginRegistryCpuLimit                             = "500m"
	DefaultPluginRegistryCpuRequest                           = "100m"

	// DevfileRegistry
	DefaultDevfileRegistryMemoryLimit   = "256Mi"
	DefaultDevfileRegistryMemoryRequest = "32Mi"
	DefaultDevfileRegistryCpuLimit      = "500m"
	DefaultDevfileRegistryCpuRequest    = "100m"

	// Server
	DefaultServerMemoryLimit               = "1024Mi"
	DefaultServerMemoryRequest             = "512Mi"
	DefaultServerCpuLimit                  = "1"
	DefaultServerCpuRequest                = "100m"
	DefaultServerLogLevel                  = "INFO"
	DefaultServerMetricsPort               = int32(8087)
	DefaultServerDebugPort                 = int32(8000)
	DefaultServerTrustStoreConfigMapName   = "ca-certs"
	DefaultProxyCredentialsSecret          = "proxy-credentials"
	DefaultGitSelfSignedCertsConfigMapName = "che-git-self-signed-cert"
	DefaultJavaOpts                        = "-XX:MaxRAMPercentage=85.0"
	DefaultSecurityContextFsGroup          = 1724
	DefaultSecurityContextRunAsUser        = 1724
	DefaultCheServiceAccountName           = "che"

	// OAuth
	BitBucketOAuthConfigClientIdFileName     = "id"
	BitBucketOAuthConfigClientSecretFileName = "secret"
	BitBucketOAuthConfigMountPath            = "/che-conf/oauth/bitbucket"
	BitBucketOAuthConfigPrivateKeyFileName   = "private.key"
	BitBucketOAuthConfigConsumerKeyFileName  = "consumer.key"
	GitHubOAuthConfigMountPath               = "/che-conf/oauth/github"
	GitHubOAuthConfigClientIdFileName        = "id"
	GitHubOAuthConfigClientSecretFileName    = "secret"
	GitLabOAuthConfigMountPath               = "/che-conf/oauth/gitlab"
	GitLabOAuthConfigClientIdFileName        = "id"
	GitLabOAuthConfigClientSecretFileName    = "secret"
	OAuthScmConfiguration                    = "oauth-scm-configuration"
	AccessToken                              = "access_token"
	IdToken                                  = "id_token"

	// Labels
	KubernetesComponentLabelKey = "app.kubernetes.io/component"
	KubernetesPartOfLabelKey    = "app.kubernetes.io/part-of"
	KubernetesManagedByLabelKey = "app.kubernetes.io/managed-by"
	KubernetesInstanceLabelKey  = "app.kubernetes.io/instance"
	KubernetesNameLabelKey      = "app.kubernetes.io/name"

	// Annotations
	CheEclipseOrgMountPath                = "che.eclipse.org/mount-path"
	CheEclipseOrgMountAs                  = "che.eclipse.org/mount-as"
	CheEclipseOrgEnvName                  = "che.eclipse.org/env-name"
	CheEclipseOrgNamespace                = "che.eclipse.org/namespace"
	CheEclipseOrgOAuthScmServer           = "che.eclipse.org/oauth-scm-server"
	CheEclipseOrgScmServerEndpoint        = "che.eclipse.org/scm-server-endpoint"
	CheEclipseOrgManagedAnnotationsDigest = "che.eclipse.org/managed-annotations-digest"

	// DevEnvironments
	PerUserPVCStorageStrategy      = "per-user"
	DefaultPvcStorageStrategy      = "per-user"
	PerWorkspacePVCStorageStrategy = "per-workspace"
	CommonPVCStorageStrategy       = "common"
	DefaultAutoProvision           = true
	DefaultWorkspaceJavaOpts       = "-XX:MaxRAM=150m -XX:MaxRAMFraction=2 -XX:+UseParallelGC " +
		"-XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 " +
		"-Dsun.zip.disableMemoryMapping=true " +
		"-Xms20m -Djava.security.egd=file:/dev/./urandom"

	// Ingress
	DefaultSelfSignedCertificateSecretName = "self-signed-certificate"
	DefaultCheTLSSecretName                = "che-tls"
	DefaultIngressClass                    = "nginx"

	// ImagePuller
	KubernetesImagePullerOperatorCSV = "kubernetes-imagepuller-operator.v0.0.9"

	// components name
	DevfileRegistryName                = "devfile-registry"
	PluginRegistryName                 = "plugin-registry"
	PostgresName                       = "postgres"
	GatewayContainerName               = "gateway"
	GatewayConfigSideCarContainerName  = "configbump"
	GatewayAuthenticationContainerName = "oauth-proxy"
	GatewayAuthorizationContainerName  = "kube-rbac-proxy"

	// common
	CheEclipseOrg         = "che.eclipse.org"
	InstallOrUpdateFailed = "InstallOrUpdateFailed"

	// DevWorkspace
	DevWorkspaceServiceAccountName = "devworkspace-controller-serviceaccount"
	DefaultContainerBuildSccName   = "container-build"
)

var (
	DefaultSingleHostGatewayConfigMapLabels = map[string]string{
		"app":       "che",
		"component": "che-gateway-config",
	}
)
