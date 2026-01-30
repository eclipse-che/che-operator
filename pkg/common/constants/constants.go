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

package constants

const (
	// Dashboard
	DefaultDashboardMemoryLimit   = "1024Mi"
	DefaultDashboardMemoryRequest = "32Mi"
	DefaultDashboardCpuLimit      = "500m"
	DefaultDashboardCpuRequest    = "100m"

	// Gateway
	DefaultGatewayMemoryLimit            = "256Mi"
	DefaultGatewayMemoryRequest          = "64Mi"
	DefaultGatewayCpuLimit               = "500m"
	DefaultGatewayCpuRequest             = "50m"
	DefaultTraefikLogLevel               = "INFO"
	DefaultKubeRbacProxyLogLevel         = int32(0)
	DefaultOAuthProxyCookieExpireSeconds = int32(86400)

	// PluginRegistry
	DefaultPluginRegistryMemoryLimit                          = "256Mi"
	DefaultPluginRegistryMemoryLimitEmbeddedOpenVSXRegistry   = "4Gi"
	DefaultPluginRegistryMemoryRequest                        = "32Mi"
	DefaultPluginRegistryMemoryRequestEmbeddedOpenVSXRegistry = "512Mi"
	DefaultPluginRegistryCpuLimit                             = "500m"
	DefaultPluginRegistryCpuRequest                           = "100m"

	// Server
	DefaultServerMemoryLimit               = "1024Mi"
	DefaultServerMemoryRequest             = "512Mi"
	DefaultServerCpuLimit                  = "1"
	DefaultServerCpuRequest                = "100m"
	DefaultServerLogLevel                  = "INFO"
	DefaultServerMetricsPort               = int32(8087)
	DefaultServerDebugPort                 = int32(8000)
	DefaultCaBundleCertsCMName             = "ca-certs"
	DefaultProxyCredentialsSecret          = "proxy-credentials"
	DefaultGitSelfSignedCertsConfigMapName = "che-git-self-signed-cert"
	DefaultJavaOpts                        = "-XX:MaxRAMPercentage=85.0"
	DefaultSecurityContextFsGroup          = 1724
	DefaultSecurityContextRunAsUser        = 1724
	DefaultCheServiceAccountName           = "che"

	// OAuth
	BitBucketOAuthConfigClientIdFileName       = "id"
	BitBucketOAuthConfigClientSecretFileName   = "secret"
	BitBucketOAuthConfigMountPath              = "/che-conf/oauth/bitbucket"
	BitBucketOAuthConfigPrivateKeyFileName     = "private.key"
	BitBucketOAuthConfigConsumerKeyFileName    = "consumer.key"
	GitHubOAuth                                = "github"
	GitHubOAuthConfigMountPath                 = "/che-conf/oauth/github"
	GitHubOAuthConfigClientIdFileName          = "id"
	GitHubOAuthConfigClientSecretFileName      = "secret"
	AzureDevOpsOAuth                           = "azure-devops"
	AzureDevOpsOAuthConfigMountPath            = "/che-conf/oauth/azure-devops"
	AzureDevOpsOAuthConfigClientIdFileName     = "id"
	AzureDevOpsOAuthConfigClientSecretFileName = "secret"
	GitlabOAuth                                = "gitlab"
	GitLabOAuthConfigMountPath                 = "/che-conf/oauth/gitlab"
	GitLabOAuthConfigClientIdFileName          = "id"
	GitLabOAuthConfigClientSecretFileName      = "secret"
	OAuthScmConfiguration                      = "oauth-scm-configuration"
	AccessToken                                = "access_token"
	IdToken                                    = "id_token"
	OpenShiftOAuthScope                        = "user:full"

	// Labels
	KubernetesComponentLabelKey = "app.kubernetes.io/component"
	KubernetesPartOfLabelKey    = "app.kubernetes.io/part-of"
	KubernetesManagedByLabelKey = "app.kubernetes.io/managed-by"
	KubernetesInstanceLabelKey  = "app.kubernetes.io/instance"
	KubernetesNameLabelKey      = "app.kubernetes.io/name"

	// Annotations
	CheEclipseOrgMountPath                          = "che.eclipse.org/mount-path"
	CheEclipseOrgMountAs                            = "che.eclipse.org/mount-as"
	CheEclipseOrgEnvName                            = "che.eclipse.org/env-name"
	CheEclipseOrgNamespace                          = "che.eclipse.org/namespace"
	CheEclipseOrgOAuthScmServer                     = "che.eclipse.org/oauth-scm-server"
	CheEclipseOrgScmServerEndpoint                  = "che.eclipse.org/scm-server-endpoint"
	CheEclipseOrgManagedAnnotationsDigest           = "che.eclipse.org/managed-annotations-digest"
	CheEclipseOrgScmGitHubDisableSubdomainIsolation = "che.eclipse.org/scm-github-disable-subdomain-isolation"
	OpenShiftIOOwningComponent                      = "openshift.io/owning-component"
	ConfigOpenShiftIOInjectTrustedCaBundle          = "config.openshift.io/inject-trusted-cabundle"

	// DevEnvironments
	PerUserPVCStorageStrategy      = "per-user"
	DefaultPvcStorageStrategy      = "per-user"
	PerWorkspacePVCStorageStrategy = "per-workspace"
	EphemeralPVCStorageStrategy    = "ephemeral"
	CommonPVCStorageStrategy       = "common"
	DefaultDeploymentStrategy      = "Recreate"
	DefaultAutoProvision           = true

	// Ingress
	DefaultSelfSignedCertificateSecretName = "self-signed-certificate"
	DefaultCheTLSSecretName                = "che-tls"
	DefaultIngressClass                    = "nginx"

	// components name
	DevfileRegistryName                = "devfile-registry"
	PluginRegistryName                 = "plugin-registry"
	GatewayContainerName               = "gateway"
	GatewayConfigSideCarContainerName  = "configbump"
	GatewayAuthenticationContainerName = "oauth-proxy"
	GatewayAuthorizationContainerName  = "kube-rbac-proxy"
	KubernetesImagePullerComponentName = "kubernetes-image-puller"
	EditorDefinitionComponentName      = "editor-definition"
	CheCABundle                        = "ca-bundle"

	// common
	CheFlavor             = "che"
	CheEclipseOrg         = "che.eclipse.org"
	WorkspacesConfig      = "workspaces-config"
	InstallOrUpdateFailed = "InstallOrUpdateFailed"
	FinalizerSuffix       = "finalizers.che.eclipse.org"
	PublicCertsDir        = "/public-certs"

	// DevWorkspace
	DevWorkspaceControllerName             = "devworkspace-controller"
	DevWorkspaceOperatorName               = "devworkspace-operator"
	DevWorkspaceServiceAccountName         = "devworkspace-controller-serviceaccount"
	DefaultContainerBuildSccName           = "container-build"
	DefaultContainerRunSccName             = "container-run"
	DefaultDisableContainerRunCapabilities = true
	DevWorkspaceOperatorConfigPlural       = "devworkspaceoperatorconfigs"
	DevWorkspaceRoutingPlural              = "devworkspaceroutings"
	DevWorkspaceOperatorNotExistsErrorMsg  = "DevWorkspace Operator is not installed. Please install it before creating a CheCluster instance"

	// TODO update on each release
	MinimumDevWorkspaceVersion = "0.38.0"

	// Finalizers
	ContainerBuildFinalizer = "container-build.finalizers.che.eclipse.org"
	ContainerRunFinalizer   = "che.eclipse.org/container-run"
)

var (
	DefaultSingleHostGatewayConfigMapLabels = map[string]string{
		"app":       "che",
		"component": "che-gateway-config",
	}
)
