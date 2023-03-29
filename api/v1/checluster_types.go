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

package v1

// Important: Don't modify this file.

import (
	"strings"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// +k8s:openapi-gen=true
// Desired configuration of the Che installation.
// Based on these settings, the Operator automatically creates and maintains
// several ConfigMaps that will contain the appropriate environment variables
// the various components of the Che installation.
// These generated ConfigMaps must NOT be updated manually.
type CheClusterSpec struct {
	// General configuration settings related to the Che server, the plugin and devfile registries
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Che server"
	Server CheClusterSpecServer `json:"server"`
	// Configuration settings related to the database used by the Che installation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Database"
	Database CheClusterSpecDB `json:"database"`
	// Configuration settings related to the Authentication used by the Che installation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Authentication"
	Auth CheClusterSpecAuth `json:"auth"`
	// Configuration settings related to the persistent storage used by the Che installation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Persistent storage"
	Storage CheClusterSpecStorage `json:"storage"`
	// Configuration settings related to the User Dashboard used by the Che installation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="User Dashboard"
	Dashboard CheClusterSpecDashboard `json:"dashboard"`
	// Configuration settings related to the metrics collection used by the Che installation.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Metrics"
	Metrics CheClusterSpecMetrics `json:"metrics"`
	// Configuration settings specific to Che installations made on upstream Kubernetes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Kubernetes"
	K8s CheClusterSpecK8SOnly `json:"k8s"`
	// Kubernetes Image Puller configuration
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Kubernetes Image Puller"
	ImagePuller CheClusterSpecImagePuller `json:"imagePuller"`
	// DevWorkspace operator configuration
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Dev Workspace operator"
	DevWorkspace CheClusterSpecDevWorkspace `json:"devWorkspace"`
	// A configuration that allows users to work with remote Git repositories.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Git Services"
	GitServices CheClusterGitServices `json:"gitServices"`
}

// +k8s:openapi-gen=true
// General configuration settings related to the Che server, the plugin and devfile registries.
type CheClusterSpecServer struct {
	// Optional host name, or URL, to an alternate container registry to pull images from.
	// This value overrides the container registry host name defined in all the default container images involved in a Che deployment.
	// This is particularly useful to install Che in a restricted environment.
	// +optional
	AirGapContainerRegistryHostname string `json:"airGapContainerRegistryHostname,omitempty"`
	// Optional repository name of an alternate container registry to pull images from.
	// This value overrides the container registry organization defined in all the default container images involved in a Che deployment.
	// This is particularly useful to install Eclipse Che in a restricted environment.
	// +optional
	AirGapContainerRegistryOrganization string `json:"airGapContainerRegistryOrganization,omitempty"`
	// Overrides the container image used in Che deployment. This does NOT include the container image tag.
	// Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	CheImage string `json:"cheImage,omitempty"`
	// Overrides the tag of the container image used in Che deployment.
	// Omit it or leave it empty to use the default image tag provided by the Operator.
	// +optional
	CheImageTag string `json:"cheImageTag,omitempty"`
	// Overrides the image pull policy used in Che deployment.
	// Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	CheImagePullPolicy corev1.PullPolicy `json:"cheImagePullPolicy,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Specifies a variation of the installation. The options are  `che` for upstream Che installations or
	// `devspaces` for Red Hat OpenShift Dev Spaces (formerly Red Hat CodeReady Workspaces) installation
	// +optional
	CheFlavor string `json:"cheFlavor,omitempty"`
	// Public host name of the installed Che server. When value is omitted, the value it will be automatically set by the Operator.
	// See the `cheHostTLSSecret` field.
	// +optional
	CheHost string `json:"cheHost,omitempty"`
	// Name of a secret containing certificates to secure ingress or route for the custom host name of the installed Che server.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// See the `cheHost` field.
	// +optional
	CheHostTLSSecret string `json:"cheHostTLSSecret,omitempty"`
	// Log level for the Che server: `INFO` or `DEBUG`. Defaults to `INFO`.
	// +optional
	CheLogLevel string `json:"cheLogLevel,omitempty"`
	// Enables the debug mode for Che server. Defaults to `false`.
	// +optional
	CheDebug string `json:"cheDebug,omitempty"`
	// A comma-separated list of ClusterRoles that will be assigned to Che ServiceAccount.
	// Each role must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// Be aware that the Che Operator has to already have all permissions in these ClusterRoles to grant them.
	// +optional
	CheClusterRoles string `json:"cheClusterRoles,omitempty"`
	// Custom cluster role bound to the user for the Che workspaces.
	// The role must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// The default roles are used when omitted or left blank.
	// +optional
	CheWorkspaceClusterRole string `json:"cheWorkspaceClusterRole,omitempty"`
	// Defines Kubernetes default namespace in which user's workspaces are created for a case when a user does not override it.
	// It's possible to use `<username>`, `<userid>` and `<workspaceid>` placeholders, such as che-workspace-<username>.
	// In that case, a new namespace will be created for each user or workspace.
	// +optional
	WorkspaceNamespaceDefault string `json:"workspaceNamespaceDefault,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Defines that a user is allowed to specify a Kubernetes namespace, or an OpenShift project, which differs from the default.
	// It's NOT RECOMMENDED to set to `true` without OpenShift OAuth configured. The OpenShift infrastructure also uses this property.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	AllowUserDefinedWorkspaceNamespaces bool `json:"allowUserDefinedWorkspaceNamespaces"`
	// Indicates if is allowed to automatically create a user namespace.
	// If it set to false, then user namespace must be pre-created by a cluster administrator.
	// +optional
	AllowAutoProvisionUserNamespace *bool `json:"allowAutoProvisionUserNamespace,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// The Che Operator will automatically detect whether the router certificate is self-signed and propagate it to other components, such as the Che server.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	SelfSignedCert bool `json:"selfSignedCert"`
	// Name of the ConfigMap with public certificates to add to Java trust store of the Che server.
	// This is often required when adding the OpenShift OAuth provider, which has HTTPS endpoint signed with self-signed cert.
	// The Che server must be aware of its CA cert to be able to request it. This is disabled by default.
	// The Config Map must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	ServerTrustStoreConfigMapName string `json:"serverTrustStoreConfigMapName,omitempty"`
	// When enabled, the certificate from `che-git-self-signed-cert` ConfigMap will be propagated to the Che components and provide particular configuration for Git.
	// Note, the `che-git-self-signed-cert` ConfigMap must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	GitSelfSignedCert bool `json:"gitSelfSignedCert"`
	// Deprecated. Instructs the Operator to deploy Che in TLS mode. This is enabled by default. Disabling TLS sometimes cause malfunction of some Che components.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	TlsSupport bool `json:"tlsSupport"`
	// Deprecated in favor of `disableInternalClusterSVCNames`.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	UseInternalClusterSVCNames bool `json:"useInternalClusterSVCNames"`
	// Deprecated. The value of this flag is ignored.
	// Disable internal cluster SVC names usage to communicate between components to speed up the traffic and avoid proxy issues.
	// +optional
	DisableInternalClusterSVCNames *bool `json:"disableInternalClusterSVCNames,omitempty"`
	// Overrides the container image used in the dashboard deployment.
	// This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	DashboardImage string `json:"dashboardImage,omitempty"`
	// Overrides the image pull policy used in the dashboard deployment.
	// Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	DashboardImagePullPolicy string `json:"dashboardImagePullPolicy,omitempty"`
	// Overrides the memory limit used in the dashboard deployment. Defaults to 256Mi.
	// +optional
	DashboardMemoryLimit string `json:"dashboardMemoryLimit,omitempty"`
	// Overrides the memory request used in the dashboard deployment. Defaults to 16Mi.
	// +optional
	DashboardMemoryRequest string `json:"dashboardMemoryRequest,omitempty"`
	// Overrides the CPU limit used in the dashboard deployment.
	// In cores. (500m = .5 cores). Default to 500m.
	// +optional
	DashboardCpuLimit string `json:"dashboardCpuLimit,omitempty"`
	// Overrides the CPU request used in the dashboard deployment.
	// In cores. (500m = .5 cores). Default to 100m.
	// +optional
	DashboardCpuRequest string `json:"dashboardCpuRequest,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Dashboard ingress custom settings.
	// +optional
	DashboardIngress IngressCustomSettings `json:"dashboardIngress,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Dashboard route custom settings.
	// +optional
	DashboardRoute RouteCustomSettings `json:"dashboardRoute,omitempty"`
	// Deprecated in favor of `externalDevfileRegistries` fields.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	DevfileRegistryUrl string `json:"devfileRegistryUrl,omitempty"`
	// Overrides the container image used in the devfile registry deployment.
	// This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	DevfileRegistryImage string `json:"devfileRegistryImage,omitempty"`
	// Overrides the image pull policy used in the devfile registry deployment.
	// Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	DevfileRegistryPullPolicy corev1.PullPolicy `json:"devfileRegistryPullPolicy,omitempty"`
	// Overrides the memory limit used in the devfile registry deployment. Defaults to 256Mi.
	// +optional
	DevfileRegistryMemoryLimit string `json:"devfileRegistryMemoryLimit,omitempty"`
	// Overrides the memory request used in the devfile registry deployment. Defaults to 16Mi.
	// +optional
	DevfileRegistryMemoryRequest string `json:"devfileRegistryMemoryRequest,omitempty"`
	// Overrides the CPU limit used in the devfile registry deployment.
	// In cores. (500m = .5 cores). Default to 500m.
	// +optional
	DevfileRegistryCpuLimit string `json:"devfileRegistryCpuLimit,omitempty"`
	// Overrides the CPU request used in the devfile registry deployment.
	// In cores. (500m = .5 cores). Default to 100m.
	// +optional
	DevfileRegistryCpuRequest string `json:"devfileRegistryCpuRequest,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// The devfile registry ingress custom settings.
	// +optional
	DevfileRegistryIngress IngressCustomSettings `json:"devfileRegistryIngress,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// The devfile registry route custom settings.
	// +optional
	DevfileRegistryRoute RouteCustomSettings `json:"devfileRegistryRoute,omitempty"`
	// Instructs the Operator on whether to deploy a dedicated devfile registry server.
	// By default, a dedicated devfile registry server is started. When `externalDevfileRegistry` is `true`,
	// no such dedicated server will be started by the Operator and configure at least one
	// devfile registry with `externalDevfileRegistries` field.
	// +optional
	ExternalDevfileRegistry bool `json:"externalDevfileRegistry"`
	// External devfile registries, that serves sample, ready-to-use devfiles.
	// Configure this in addition to a dedicated devfile registry (when `externalDevfileRegistry` is `false`)
	// or instead of it (when `externalDevfileRegistry` is `true`)
	// +optional
	ExternalDevfileRegistries []ExternalDevfileRegistries `json:"externalDevfileRegistries,omitempty"`
	// Public URL of the plugin registry that serves sample ready-to-use devfiles.
	// Set this ONLY when a use of an external devfile registry is needed.
	// See the `externalPluginRegistry` field. By default, this will be automatically calculated by the Operator.
	// +optional
	PluginRegistryUrl string `json:"pluginRegistryUrl,omitempty"`
	// Overrides the container image used in the plugin registry deployment.
	// This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	PluginRegistryImage string `json:"pluginRegistryImage,omitempty"`
	// Overrides the image pull policy used in the plugin registry deployment.
	// Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	PluginRegistryPullPolicy corev1.PullPolicy `json:"pluginRegistryPullPolicy,omitempty"`
	// Overrides the memory limit used in the plugin registry deployment. Defaults to 1536Mi.
	// +optional
	PluginRegistryMemoryLimit string `json:"pluginRegistryMemoryLimit,omitempty"`
	// Overrides the memory request used in the plugin registry deployment. Defaults to 16Mi.
	// +optional
	PluginRegistryMemoryRequest string `json:"pluginRegistryMemoryRequest,omitempty"`
	// Overrides the CPU limit used in the plugin registry deployment.
	// In cores. (500m = .5 cores). Default to 500m.
	// +optional
	PluginRegistryCpuLimit string `json:"pluginRegistryCpuLimit,omitempty"`
	// Overrides the CPU request used in the plugin registry deployment.
	// In cores. (500m = .5 cores). Default to 100m.
	// +optional
	PluginRegistryCpuRequest string `json:"pluginRegistryCpuRequest,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Plugin registry ingress custom settings.
	// +optional
	PluginRegistryIngress IngressCustomSettings `json:"pluginRegistryIngress,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Plugin registry route custom settings.
	// +optional
	PluginRegistryRoute RouteCustomSettings `json:"pluginRegistryRoute,omitempty"`
	// Instructs the Operator on whether to deploy a dedicated plugin registry server.
	// By default, a dedicated plugin registry server is started. When `externalPluginRegistry` is `true`, no such dedicated server
	// will be started by the Operator and you will have to manually set the `pluginRegistryUrl` field.
	// +optional
	ExternalPluginRegistry bool `json:"externalPluginRegistry"`
	// Map of additional environment variables that will be applied in the generated `che` ConfigMap to be used by the Che server,
	// in addition to the values already generated from other fields of the `CheCluster` custom resource (CR).
	// When `customCheProperties` contains a property that would be normally generated in `che` ConfigMap from other CR fields,
	// the value defined in the `customCheProperties` is used instead.
	// +optional
	CustomCheProperties map[string]string `json:"customCheProperties,omitempty"`
	// URL (protocol+host name) of the proxy server. This drives the appropriate changes in the `JAVA_OPTS` and `https(s)_proxy` variables
	// in the Che server and workspaces containers.
	// Only use when configuring a proxy is required. Operator respects OpenShift cluster wide proxy configuration
	// and no additional configuration is required, but defining `proxyUrl` in a custom resource leads to overrides the cluster proxy configuration
	// with fields `proxyUrl`, `proxyPort`, `proxyUser` and `proxyPassword` from the custom resource.
	// See the doc https://docs.openshift.com/container-platform/4.4/networking/enable-cluster-wide-proxy.html. See also the `proxyPort` and `nonProxyHosts` fields.
	// +optional
	ProxyURL string `json:"proxyURL,omitempty"`
	// Port of the proxy server. Only use when configuring a proxy is required. See also the `proxyURL` and `nonProxyHosts` fields.
	// +optional
	ProxyPort string `json:"proxyPort,omitempty"`
	// List of hosts that will be reached directly, bypassing the proxy.
	// Specify wild card domain use the following form `.<DOMAIN>` and `|` as delimiter, for example: `localhost|.my.host.com|123.42.12.32`
	// Only use when configuring a proxy is required. Operator respects OpenShift cluster wide proxy configuration and no additional configuration is required,
	// but defining `nonProxyHosts` in a custom resource leads to merging non proxy hosts lists from the cluster proxy configuration and ones defined in the custom resources.
	// See the doc https://docs.openshift.com/container-platform/4.4/networking/enable-cluster-wide-proxy.html. See also the `proxyURL` fields.
	NonProxyHosts string `json:"nonProxyHosts,omitempty"`
	// User name of the proxy server. Only use when configuring a proxy is required. See also the `proxyURL`, `proxyPassword` and `proxySecret` fields.
	// +optional
	ProxyUser string `json:"proxyUser,omitempty"`
	// Password of the proxy server.
	// Only use when proxy configuration is required. See the `proxyURL`, `proxyUser` and `proxySecret` fields.
	// +optional
	ProxyPassword string `json:"proxyPassword,omitempty"`
	// The secret that contains `user` and `password` for a proxy server. When the secret is defined, the `proxyUser` and `proxyPassword` are ignored.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	ProxySecret string `json:"proxySecret,omitempty"`
	// Overrides the memory request used in the Che server deployment. Defaults to 512Mi.
	// +optional
	ServerMemoryRequest string `json:"serverMemoryRequest,omitempty"`
	// Overrides the memory limit used in the Che server deployment. Defaults to 1Gi.
	// +optional
	ServerMemoryLimit string `json:"serverMemoryLimit,omitempty"`
	// Overrides the CPU limit used in the Che server deployment
	// In cores. (500m = .5 cores). Default to 1.
	// +optional
	ServerCpuLimit string `json:"serverCpuLimit,omitempty"`
	// Overrides the CPU request used in the Che server deployment
	// In cores. (500m = .5 cores). Default to 100m.
	// +optional
	ServerCpuRequest string `json:"serverCpuRequest,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Sets the server and workspaces exposure type.
	// Possible values are `multi-host`, `single-host`, `default-host`. Defaults to `multi-host`, which creates a separate ingress, or OpenShift routes, for every required endpoint.
	// `single-host` makes Che exposed on a single host name with workspaces exposed on subpaths.
	// Read the docs to learn about the limitations of this approach.
	// Also consult the `singleHostExposureType` property to further configure how the Operator and the Che server make that happen on Kubernetes.
	// `default-host` exposes the Che server on the host of the cluster. Read the docs to learn about the limitations of this approach.
	// +optional
	ServerExposureStrategy string `json:"serverExposureStrategy,omitempty"`
	// The image used for the gateway in the single host mode. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	SingleHostGatewayImage string `json:"singleHostGatewayImage,omitempty"`
	// The image used for the gateway sidecar that provides configuration to the gateway. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	SingleHostGatewayConfigSidecarImage string `json:"singleHostGatewayConfigSidecarImage,omitempty"`
	// The labels that need to be present in the ConfigMaps representing the gateway configuration.
	// +optional
	SingleHostGatewayConfigMapLabels labels.Set `json:"singleHostGatewayConfigMapLabels,omitempty"`
	// The Che server ingress custom settings.
	// +optional
	CheServerIngress IngressCustomSettings `json:"cheServerIngress,omitempty"`
	// The Che server route custom settings.
	// +optional
	CheServerRoute RouteCustomSettings `json:"cheServerRoute,omitempty"`
	// Default plug-ins applied to Devworkspaces.
	// +optional
	WorkspacesDefaultPlugins []WorkspacesDefaultPlugins `json:"workspacesDefaultPlugins,omitempty"`
	// The node selector that limits the nodes that can run the workspace pods.
	WorkspacePodNodeSelector map[string]string `json:"workspacePodNodeSelector,omitempty"`
	// The pod tolerations put on the workspace pods to limit where the workspace pods can run.
	WorkspacePodTolerations []corev1.Toleration `json:"workspacePodTolerations,omitempty"`
	// The default editor to workspace create with. It could be a plugin ID or a URI.
	// The plugin ID must have `publisher/plugin/version`.
	// The URI must start from `http`.
	// +optional
	// +kubebuilder:default:=che-incubator/che-code/latest
	WorkspaceDefaultEditor string `json:"workspaceDefaultEditor,omitempty"`
	// Default components applied to DevWorkspaces.
	// These default components are meant to be used when a Devfile does not contain any components.
	// +optional
	WorkspaceDefaultComponents []devfile.Component `json:"workspaceDefaultComponents,omitempty"`
	// List of environment variables to set in the Che server container.
	// +optional
	CheServerEnv []corev1.EnvVar `json:"cheServerEnv,omitempty"`
	// List of environment variables to set in the plugin registry container.
	// +optional
	DevfileRegistryEnv []corev1.EnvVar `json:"devfileRegistryEnv,omitempty"`
	// List of environment variables to set in the devfile registry container.
	// +optional
	PluginRegistryEnv []corev1.EnvVar `json:"pluginRegistryEnv,omitempty"`
	// List of environment variables to set in the dashboard container.
	// +optional
	DashboardEnv []corev1.EnvVar `json:"dashboardEnv,omitempty"`
	// Open VSX registry URL. If omitted an embedded instance will be used.
	// +optional
	OpenVSXRegistryURL *string `json:"openVSXRegistryURL,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the database used by the Che installation.
type CheClusterSpecDB struct {
	// Instructs the Operator on whether to deploy a dedicated database.
	// By default, a dedicated PostgreSQL database is deployed as part of the Che installation. When `externalDb` is `true`, no dedicated database will be deployed by the
	// Operator and you will need to provide connection details to the external DB you are about to use. See also all the fields starting with: `chePostgres`.
	// +optional
	ExternalDb bool `json:"externalDb"`
	// PostgreSQL Database host name that the Che server uses to connect to.
	// Defaults is `postgres`. Override this value ONLY when using an external database. See field `externalDb`.
	// In the default case it will be automatically set by the Operator.
	// +optional
	ChePostgresHostName string `json:"chePostgresHostName,omitempty"`
	// PostgreSQL Database port that the Che server uses to connect to. Defaults to 5432.
	// Override this value ONLY when using an external database. See field `externalDb`. In the default case it will be automatically set by the Operator.
	// +optional
	ChePostgresPort string `json:"chePostgresPort,omitempty"`
	// PostgreSQL user that the Che server uses to connect to the DB. Defaults to `pgche`.
	// +optional
	ChePostgresUser string `json:"chePostgresUser,omitempty"`
	// PostgreSQL password that the Che server uses to connect to the DB. When omitted or left blank, it will be set to an automatically generated value.
	// +optional
	ChePostgresPassword string `json:"chePostgresPassword,omitempty"`
	// PostgreSQL database name that the Che server uses to connect to the DB. Defaults to `dbche`.
	// +optional
	ChePostgresDb string `json:"chePostgresDb,omitempty"`
	// The secret that contains PostgreSQL`user` and `password` that the Che server uses to connect to the DB.
	// When the secret is defined, the `chePostgresUser` and `chePostgresPassword` are ignored.
	// When the value is omitted or left blank, the one of following scenarios applies:
	// 1. `chePostgresUser` and `chePostgresPassword` are defined, then they will be used to connect to the DB.
	// 2. `chePostgresUser` or `chePostgresPassword` are not defined, then a new secret with the name `postgres-credentials`
	// will be created with default value of `pgche` for `user` and with an auto-generated value for `password`.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	ChePostgresSecret string `json:"chePostgresSecret,omitempty"`
	// Overrides the container image used in the PostgreSQL database deployment. This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	PostgresImage string `json:"postgresImage,omitempty"`
	// Indicates a PostgreSQL version image to use. Allowed values are: `9.6` and `13.3`.
	// Migrate your PostgreSQL database to switch from one version to another.
	// +optional
	PostgresVersion string `json:"postgresVersion,omitempty"`
	// Overrides the image pull policy used in the PostgreSQL database deployment. Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	PostgresImagePullPolicy corev1.PullPolicy `json:"postgresImagePullPolicy,omitempty"`
	// PostgreSQL container custom settings
	// +optional
	ChePostgresContainerResources ResourcesCustomSettings `json:"chePostgresContainerResources,omitempty"`
	// Size of the persistent volume claim for database. Defaults to `1Gi`.
	// To update pvc storageclass that provisions it must support resize when Eclipse Che has been already deployed.
	// +optional
	PvcClaimSize string `json:"pvcClaimSize,omitempty"`
	// List of environment variables to set in the PostgreSQL container.
	// +optional
	PostgresEnv []corev1.EnvVar `json:"postgresEnv,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the Authentication used by the Che installation.
type CheClusterSpecAuth struct {
	// Deprecated. The value of this flag is ignored.
	// For operating with the OpenShift OAuth authentication, create a new user account since the kubeadmin can not be used.
	// If the value is true, then a new OpenShift OAuth user will be created for the HTPasswd identity provider.
	// If the value is false and the user has already been created, then it will be removed.
	// If value is an empty, then do nothing.
	// The user's credentials are stored in the `openshift-oauth-user-credentials` secret in 'openshift-config' namespace by Operator.
	// Note that this solution is Openshift 4 platform-specific.
	InitialOpenShiftOAuthUser *bool `json:"initialOpenShiftOAuthUser,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Instructs the Operator on whether or not to deploy a dedicated Identity Provider (Keycloak or RH SSO instance).
	// Instructs the Operator on whether to deploy a dedicated Identity Provider (Keycloak or RH-SSO instance).
	// By default, a dedicated Identity Provider server is deployed as part of the Che installation. When `externalIdentityProvider` is `true`,
	// no dedicated identity provider will be deployed by the Operator and you will need to provide details about the external identity provider you are about to use.
	// See also all the other fields starting with: `identityProvider`.
	// +optional
	ExternalIdentityProvider bool `json:"externalIdentityProvider"`
	// Public URL of the Identity Provider server (Keycloak / RH-SSO server).
	// Set this ONLY when a use of an external Identity Provider is needed.
	// See the `externalIdentityProvider` field. By default, this will be automatically calculated and set by the Operator.
	// +optional
	IdentityProviderURL string `json:"identityProviderURL,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Overrides the name of the Identity Provider administrator user. Defaults to `admin`.
	// +optional
	IdentityProviderAdminUserName string `json:"identityProviderAdminUserName,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Overrides the password of Keycloak administrator user.
	// Override this when an external Identity Provider is in use. See the `externalIdentityProvider` field.
	// When omitted or left blank, it is set to an auto-generated password.
	// +optional
	IdentityProviderPassword string `json:"identityProviderPassword,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// The secret that contains `user` and `password` for Identity Provider.
	// When the secret is defined, the `identityProviderAdminUserName` and `identityProviderPassword` are ignored.
	// When the value is omitted or left blank, the one of following scenarios applies:
	// 1. `identityProviderAdminUserName` and `identityProviderPassword` are defined, then they will be used.
	// 2. `identityProviderAdminUserName` or `identityProviderPassword` are not defined, then a new secret with the name
	// `che-identity-secret` will be created with default value `admin` for `user` and with an auto-generated value for `password`.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	IdentityProviderSecret string `json:"identityProviderSecret,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Name of a Identity provider, Keycloak or RH-SSO, realm that is used for Che.
	// Override this when an external Identity Provider is in use. See the `externalIdentityProvider` field.
	// When omitted or left blank, it is set to the value of the `flavour` field.
	// +optional
	IdentityProviderRealm string `json:"identityProviderRealm,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Name of a Identity provider, Keycloak or RH-SSO, `client-id` that is used for Che.
	// Override this when an external Identity Provider is in use. See the `externalIdentityProvider` field.
	// When omitted or left blank, it is set to the value of the `flavour` field suffixed with `-public`.
	// +optional
	IdentityProviderClientId string `json:"identityProviderClientId,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Password for a Identity Provider, Keycloak or RH-SSO, to connect to the database.
	// Override this when an external Identity Provider is in use. See the `externalIdentityProvider` field.
	// When omitted or left blank, it is set to an auto-generated password.
	// +optional
	IdentityProviderPostgresPassword string `json:"identityProviderPostgresPassword,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// The secret that contains `password` for the Identity Provider, Keycloak or RH-SSO, to connect to the database.
	// When the secret is defined, the `identityProviderPostgresPassword` is ignored. When the value is omitted or left blank, the one of following scenarios applies:
	// 1. `identityProviderPostgresPassword` is defined, then it will be used to connect to the database.
	// 2. `identityProviderPostgresPassword` is not defined, then a new secret with the name `che-identity-postgres-secret` will be created with an auto-generated value for `password`.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	IdentityProviderPostgresSecret string `json:"identityProviderPostgresSecret,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Forces the default `admin` Che user to update password on first login. Defaults to `false`.
	// +optional
	UpdateAdminPassword bool `json:"updateAdminPassword"`
	// Deprecated. The value of this flag is ignored.
	// Enables the integration of the identity provider (Keycloak / RHSSO) with OpenShift OAuth.
	// Empty value on OpenShift by default. This will allow users to directly login with their OpenShift user through the OpenShift login,
	// and have their workspaces created under personal OpenShift namespaces.
	// WARNING: the `kubeadmin` user is NOT supported, and logging through it will NOT allow accessing the Che Dashboard.
	// +optional
	OpenShiftoAuth *bool `json:"openShiftoAuth,omitempty"`
	// Name of the OpenShift `OAuthClient` resource used to setup identity federation on the OpenShift side. Auto-generated when left blank. See also the `OpenShiftoAuth` field.
	// +optional
	OAuthClientName string `json:"oAuthClientName,omitempty"`
	// Name of the secret set in the OpenShift `OAuthClient` resource used to setup identity federation on the OpenShift side. Auto-generated when left blank. See also the `OAuthClientName` field.
	// +optional
	OAuthSecret string `json:"oAuthSecret,omitempty"`
	// Access Token Scope.
	// This field is specific to Che installations made for Kubernetes only and ignored for OpenShift.
	// +optional
	OAuthScope string `json:"oAuthScope,omitempty"`
	// Identity token to be passed to upstream. There are two types of tokens supported: `id_token` and `access_token`.
	// Default value is `id_token`.
	// This field is specific to Che installations made for Kubernetes only and ignored for OpenShift.
	// +optional
	IdentityToken string `json:"identityToken,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Overrides the container image used in the Identity Provider, Keycloak or RH-SSO, deployment.
	// This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	IdentityProviderImage string `json:"identityProviderImage,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Overrides the image pull policy used in the Identity Provider, Keycloak or RH-SSO, deployment.
	// Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	IdentityProviderImagePullPolicy corev1.PullPolicy `json:"identityProviderImagePullPolicy,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Ingress custom settings.
	// +optional
	IdentityProviderIngress IngressCustomSettings `json:"identityProviderIngress,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Route custom settings.
	// +optional
	IdentityProviderRoute RouteCustomSettings `json:"identityProviderRoute,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Identity provider container custom settings.
	// +optional
	IdentityProviderContainerResources ResourcesCustomSettings `json:"identityProviderContainerResources,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Enables native user mode. Currently works only on OpenShift and DevWorkspace engine.
	// Native User mode uses OpenShift OAuth directly as identity provider, without Keycloak.
	// +optional
	NativeUserMode *bool `json:"nativeUserMode,omitempty"`
	// Gateway sidecar responsible for authentication when NativeUserMode is enabled.
	// See link:https://github.com/oauth2-proxy/oauth2-proxy[oauth2-proxy] or link:https://github.com/openshift/oauth-proxy[openshift/oauth-proxy].
	// +optional
	GatewayAuthenticationSidecarImage string `json:"gatewayAuthenticationSidecarImage,omitempty"`
	// Gateway sidecar responsible for authorization when NativeUserMode is enabled.
	// See link:https://github.com/brancz/kube-rbac-proxy[kube-rbac-proxy] or link:https://github.com/openshift/kube-rbac-proxy[openshift/kube-rbac-proxy]
	// +optional
	GatewayAuthorizationSidecarImage string `json:"gatewayAuthorizationSidecarImage,omitempty"`
	// Deprecated. The value of this flag is ignored. Sidecar functionality is now implemented in Traefik plugin.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:hidden"
	GatewayHeaderRewriteSidecarImage string `json:"gatewayHeaderRewriteSidecarImage,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Debug internal identity provider.
	Debug bool `json:"debug,omitempty"`
	// List of environment variables to set in the Gateway container.
	// +optional
	GatewayEnv []corev1.EnvVar `json:"gatewayEnv,omitempty"`
	// List of environment variables to set in the Configbump container.
	// +optional
	GatewayConfigBumpEnv []corev1.EnvVar `json:"gatewayConfigBumpEnv,omitempty"`
	// List of environment variables to set in the OAuth proxy container.
	// +optional
	GatewayOAuthProxyEnv []corev1.EnvVar `json:"gatewayOAuthProxyEnv,omitempty"`
	// List of environment variables to set in the Kube rbac proxy container.
	// +optional
	GatewayKubeRbacProxyEnv []corev1.EnvVar `json:"gatewayKubeRbacProxyEnv,omitempty"`
}

// Ingress custom settings, can be extended in the future
type IngressCustomSettings struct {
	// Comma separated list of labels that can be used to organize and categorize objects by scoping and selecting.
	// +optional
	Labels string `json:"labels,omitempty"`
	// Unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Route custom settings, can be extended in the future
type RouteCustomSettings struct {
	// Comma separated list of labels that can be used to organize and categorize objects by scoping and selecting.
	// +optional
	Labels string `json:"labels,omitempty"`
	// Operator uses the domain to generate a hostname for a route.
	// In a conjunction with labels it creates a route, which is served by a non-default Ingress controller.
	// The generated host name will follow this pattern: `<route-name>-<route-namespace>.<domain>`.
	// +optional
	Domain string `json:"domain,omitempty"`
	// Unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ResourceRequirements describes the compute resource requirements.
type ResourcesCustomSettings struct {
	// Requests describes the minimum amount of compute resources required.
	// +optional
	Requests Resources `json:"request,omitempty"`
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits Resources `json:"limits,omitempty"`
}

type WorkspacesDefaultPlugins struct {
	// The editor id to specify default plug-ins for.
	Editor string `json:"editor,omitempty"`
	// Default plug-in uris for the specified editor.
	Plugins []string `json:"plugins,omitempty"`
}

// List of resources
type Resources struct {
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// +optional
	Memory string `json:"memory,omitempty"`
	// CPU, in cores. (500m = .5 cores)
	// +optional
	Cpu string `json:"cpu,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the User Dashboard used by the Che installation.
type CheClusterSpecDashboard struct {
	// Warning message that will be displayed on the User Dashboard
	// +optional
	Warning string `json:"warning,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the persistent storage used by the Che installation.
type CheClusterSpecStorage struct {
	// Persistent volume claim strategy for the Che server. This Can be:`common` (all workspaces PVCs in one volume),
	// `per-workspace` (one PVC per workspace for all declared volumes) and `unique` (one PVC per declared volume). Defaults to `common`.
	// +optional
	PvcStrategy string `json:"pvcStrategy,omitempty"`
	// Size of the persistent volume claim for workspaces. Defaults to `10Gi`.
	// +optional
	PvcClaimSize string `json:"pvcClaimSize,omitempty"`
	// Instructs the Che server to start a special Pod to pre-create a sub-path in the Persistent Volumes.
	// Defaults to `false`, however it will need to enable it according to the configuration of your Kubernetes cluster.
	// +optional
	PreCreateSubPaths bool `json:"preCreateSubPaths"`
	// Overrides the container image used to create sub-paths in the Persistent Volumes.
	// This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator. See also the `preCreateSubPaths` field.
	// +optional
	PvcJobsImage string `json:"pvcJobsImage,omitempty"`
	// Storage class for the Persistent Volume Claim dedicated to the PostgreSQL database. When omitted or left blank, a default storage class is used.
	// +optional
	PostgresPVCStorageClassName string `json:"postgresPVCStorageClassName,omitempty"`
	// Storage class for the Persistent Volume Claims dedicated to the Che workspaces. When omitted or left blank, a default storage class is used.
	// +optional
	WorkspacePVCStorageClassName string `json:"workspacePVCStorageClassName,omitempty"`
	// Size of the persistent volume claim for workspaces.
	// +optional
	PerWorkspaceStrategyPvcClaimSize string `json:"perWorkspaceStrategyPvcClaimSize,omitempty"`
	// Storage class for the Persistent Volume Claims dedicated to the Che workspaces. When omitted or left blank, a default storage class is used.
	// +optional
	PerWorkspaceStrategyPVCStorageClassName string `json:"perWorkspaceStrategyPVCStorageClassName,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings specific to Che installations made on upstream Kubernetes.
type CheClusterSpecK8SOnly struct {
	// Global ingress domain for a Kubernetes cluster. This MUST be explicitly specified: there are no defaults.
	IngressDomain string `json:"ingressDomain,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// Strategy for ingress creation. Options are: `multi-host` (host is explicitly provided in ingress),
	// `single-host` (host is provided, path-based rules) and `default-host` (no host is provided, path-based rules).
	// Defaults to `multi-host` Deprecated in favor of `serverExposureStrategy` in the `server` section,
	// which defines this regardless of the cluster type. When both are defined, the `serverExposureStrategy` option takes precedence.
	// +optional
	IngressStrategy string `json:"ingressStrategy,omitempty"`
	// Ingress class that will define the which controller will manage ingresses. Defaults to `nginx`.
	// NB: This drives the `kubernetes.io/ingress.class` annotation on Che-related ingresses.
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`
	// Name of a secret that will be used to setup ingress TLS termination when TLS is enabled.
	// When the field is empty string, the default cluster certificate will be used. See also the `tlsSupport` field.
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
	// The FSGroup in which the Che Pod and workspace Pods containers runs in. Default value is `1724`.
	// +optional
	SecurityContextFsGroup string `json:"securityContextFsGroup,omitempty"`
	// ID of the user the Che Pod and workspace Pods containers run as. Default value is `1724`.
	// +optional
	SecurityContextRunAsUser string `json:"securityContextRunAsUser,omitempty"`
	// Deprecated. The value of this flag is ignored.
	// When the serverExposureStrategy is set to `single-host`, the way the server, registries and workspaces are exposed is further configured by this property.
	// The possible values are `native`, which means that the server and workspaces are exposed using ingresses on K8s
	// or `gateway` where the server and workspaces are exposed using a custom gateway based on link:https://doc.traefik.io/traefik/[Traefik].
	// All the endpoints whether backed by the ingress or gateway `route` always point to the subpaths on the same domain. Defaults to `native`.
	// +optional
	SingleHostExposureType string `json:"singleHostExposureType,omitempty"`
}

type CheClusterSpecMetrics struct {
	// Enables `metrics` the Che server endpoint. Default to `true`.
	// +optional
	Enable bool `json:"enable"`
}

// +k8s:openapi-gen=true
// Configuration settings for installation and configuration of the Kubernetes Image Puller
// See https://github.com/che-incubator/kubernetes-image-puller-operator
type CheClusterSpecImagePuller struct {
	// Install and configure the Community Supported Kubernetes Image Puller Operator. When set to `true` and no spec is provided,
	// it will create a default KubernetesImagePuller object to be managed by the Operator.
	// When set to `false`, the KubernetesImagePuller object will be deleted, and the Operator will be uninstalled,
	// regardless of whether a spec is provided.
	// If the `spec.images` field is empty, a set of recommended workspace-related images will be automatically detected and
	// pre-pulled after installation.
	// Note that while this Operator and its behavior is community-supported, its payload may be commercially-supported
	// for pulling commercially-supported images.
	Enable bool `json:"enable"`
	// A KubernetesImagePullerSpec to configure the image puller in the CheCluster
	// +optional
	Spec chev1alpha1.KubernetesImagePullerSpec `json:"spec"`
}

// +k8s:openapi-gen=true
// Settings for installation and configuration of the DevWorkspace operator
// See https://github.com/devfile/devworkspace-operator
type CheClusterSpecDevWorkspace struct {
	// Deploys the DevWorkspace Operator in the cluster.
	// Does nothing when a matching version of the Operator is already installed.
	// Fails when a non-matching version of the Operator is already installed.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enable DevWorkspace operator"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	Enable bool `json:"enable"`
	// Overrides the container image used in the DevWorkspace controller deployment.
	// This includes the image tag. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	ControllerImage string `json:"controllerImage,omitempty"`
	// Maximum number of the running workspaces per user.
	// +optional
	RunningLimit string `json:"runningLimit,omitempty"`
	// Idle timeout for workspaces in seconds.
	// This timeout is the duration after which a workspace will be idled if there is no activity.
	// To disable workspace idling due to inactivity, set this value to -1.
	// +kubebuilder:default:=1800
	SecondsOfInactivityBeforeIdling *int32 `json:"secondsOfInactivityBeforeIdling,omitempty"`
	// Run timeout for workspaces in seconds.
	// This timeout is the maximum duration a workspace runs.
	// To disable workspace run timeout, set this value to -1.
	// +kubebuilder:default:=-1
	SecondsOfRunBeforeIdling *int32 `json:"secondsOfRunBeforeIdling,omitempty"`
	// List of environment variables to set in the DevWorkspace container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// +k8s:openapi-gen=true
// Settings for a configuration of the external devfile registries.
type ExternalDevfileRegistries struct {
	// Public URL of the devfile registry.
	// +optional
	Url string `json:"url,omitempty"`
}

type CheClusterGitServices struct {
	// Enables users to work with repositories hosted on GitHub (github.com or GitHub Enterprise).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="GitHub"
	GitHub []GitHubService `json:"github,omitempty"`
	// Enables users to work with repositories hosted on GitLab (gitlab.com or self-hosted).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="GitLab"
	GitLab []GitLabService `json:"gitlab,omitempty"`
	// Enables users to work with repositories hosted on Bitbucket (bitbucket.org or self-hosted).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Bitbucket"
	BitBucket []BitBucketService `json:"bitbucket,omitempty"`
}

// GitHubService enables users to work with repositories hosted on GitHub (GitHub.com or GitHub Enterprise).
type GitHubService struct {
	// Kubernetes secret, that contains Base64-encoded GitHub OAuth Client id and GitHub OAuth Client secret.
	// See the following page for details: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-github/.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:io.kubernetes:Secret"
	SecretName string `json:"secretName"`
	// GitHub server endpoint URL.
	// Deprecated in favor of `che.eclipse.org/scm-server-endpoint` annotation.
	// See the following page for details: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-github/.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// GitLabService enables users to work with repositories hosted on GitLab (gitlab.com or self-hosted).
type GitLabService struct {
	// Kubernetes secret, that contains Base64-encoded GitHub Application id and GitLab Application Client secret.
	// See the following page: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-gitlab/.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:io.kubernetes:Secret"
	SecretName string `json:"secretName"`
	// GitLab server endpoint URL.
	// Deprecated in favor of `che.eclipse.org/scm-server-endpoint` annotation.
	// See the following page: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-gitlab/.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// BitBucketService enables users to work with repositories hosted on Bitbucket (bitbucket.org or self-hosted).
type BitBucketService struct {
	// Kubernetes secret, that contains Base64-encoded Bitbucket OAuth 1.0 or OAuth 2.0 data.
	// See the following pages for details: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-1-for-a-bitbucket-server/
	// and https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-the-bitbucket-cloud/.
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:io.kubernetes:Secret"
	SecretName string `json:"secretName"`
	// Bitbucket server endpoint URL.
	// Deprecated in favor of `che.eclipse.org/scm-server-endpoint` annotation.
	// See the following page: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-1-for-a-bitbucket-server/.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// GatewayPhase describes the different phases of the Che gateway lifecycle
type GatewayPhase string

const (
	GatewayPhaseInitializing = "Initializing"
	GatewayPhaseEstablished  = "Established"
	GatewayPhaseInactive     = "Inactive"
)

// ClusterPhase describes the different phases of the Che cluster lifecycle
type ClusterPhase string

const (
	ClusterPhaseActive          = "Active"
	ClusterPhaseInactive        = "Inactive"
	ClusterPhasePendingDeletion = "PendingDeletion"
)

// LegacyDevworkspaceStatus contains the status of the CheCluster object
// +k8s:openapi-gen=true
type LegacyDevworkspaceStatus struct {
	// GatewayPhase specifies the phase in which the gateway deployment currently is.
	// If the gateway is disabled, the phase is "Inactive".
	GatewayPhase GatewayPhase `json:"gatewayPhase,omitempty"`

	// GatewayHost is the resolved host of the ingress/route. This is equal to the Host in the spec
	// on Kubernetes but contains the actual host name of the route if Host is unspecified on OpenShift.
	GatewayHost string `json:"gatewayHost,omitempty"`

	// Phase is the phase in which the Che cluster as a whole finds itself in.
	Phase ClusterPhase `json:"phase,omitempty"`

	// A brief CamelCase message indicating details about why the Che cluster is in this state.
	Reason string `json:"reason,omitempty"`

	// Message contains further human-readable info for why the Che cluster is in the phase it currently is.
	Message string `json:"message,omitempty"`

	// The resolved workspace base domain. This is either the copy of the explicitly defined property of the
	// same name in the spec or, if it is undefined in the spec and we're running on OpenShift, the automatically
	// resolved basedomain for routes.
	WorkspaceBaseDomain string `json:"workspaceBaseDomain,omitempty"`
}

// CheClusterStatus defines the observed state of Che installation
type CheClusterStatus struct {
	// OpenShift OAuth secret in `openshift-config` namespace that contains user credentials for HTPasswd identity provider.
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="OpenShift OAuth secret in `openshift-config` namespace that contains user credentials for HTPasswd identity provider."
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	OpenShiftOAuthUserCredentialsSecret string `json:"openShiftOAuthUserCredentialsSecret,omitempty"`
	// Indicates that a PostgreSQL instance has been correctly provisioned or not.
	// +optional
	DbProvisoned bool `json:"dbProvisioned"`
	// Indicates whether an Identity Provider instance, Keycloak or RH-SSO, has been provisioned with realm, client and user.
	// +optional
	KeycloakProvisoned bool `json:"keycloakProvisioned"`
	// Indicates whether an Identity Provider instance, Keycloak or RH-SSO, has been configured to integrate with the OpenShift OAuth.
	// +optional
	OpenShiftoAuthProvisioned bool `json:"openShiftoAuthProvisioned"`
	// Indicates whether an Identity Provider instance, Keycloak or RH-SSO, has been configured to integrate with the GitHub OAuth.
	// +optional
	GitHubOAuthProvisioned bool `json:"gitHubOAuthProvisioned"`
	// The ConfigMap containing certificates to propagate to the Che components and to provide particular configuration for Git.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Git certificates"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	GitServerTLSCertificateConfigMapName string `json:"gitServerTLSCertificateConfigMapName"`
	// Status of a Che installation. Can be `Available`, `Unavailable`, or `Available, Rolling Update in Progress`.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Status"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:io.kubernetes.phase"
	CheClusterRunning string `json:"cheClusterRunning"`
	// Current installed Che version.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="displayName: Eclipse Che version"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	CheVersion string `json:"cheVersion"`
	// Public URL to the Che server.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Eclipse Che URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	CheURL string `json:"cheURL"`
	// Public URL to the Identity Provider server, Keycloak or RH-SSO,.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Keycloak Admin Console URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	KeycloakURL string `json:"keycloakURL"`
	// Public URL to the devfile registry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Devfile registry URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	DevfileRegistryURL string `json:"devfileRegistryURL"`
	// Public URL to the plugin registry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Plugin registry URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	PluginRegistryURL string `json:"pluginRegistryURL"`
	// A human readable message indicating details about why the Pod is in this condition.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Message"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the Pod is in this state.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Reason"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	Reason string `json:"reason,omitempty"`
	// A URL that points to some URL where to find help related to the current Operator status.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Help link"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	HelpLink string `json:"helpLink,omitempty"`
	// The status of the Devworkspace subsystem
	// +optional
	DevworkspaceStatus LegacyDevworkspaceStatus `json:"devworkspaceStatus,omitempty"`
}

// The `CheCluster` custom resource allows defining and managing a Che server installation
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="org.eclipse.che/v1 CheCluster is deprecated and will be removed in future releases"
// +k8s:openapi-gen=true
// +operator-sdk:csv:customresourcedefinitions:displayName="Eclipse Che instance Specification"
// +operator-sdk:csv:customresourcedefinitions:order=1
// +operator-sdk:csv:customresourcedefinitions:resources={{Ingress,v1},{Route,v1},{ConfigMap,v1},{Service,v1},{Secret,v1},{Deployment,apps/v1},{Role,v1},{RoleBinding,v1},{ClusterRole,v1},{ClusterRoleBinding,v1}}
type CheCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Desired configuration of the Che installation.
	// Based on these settings, the  Operator automatically creates and maintains
	// several ConfigMaps that will contain the appropriate environment variables
	// the various components of the Che installation.
	// These generated ConfigMaps must NOT be updated manually.
	Spec CheClusterSpec `json:"spec,omitempty"`

	// CheClusterStatus defines the observed state of Che installation
	Status CheClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// CheClusterList contains a list of CheCluster
type CheClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheCluster{}, &CheClusterList{})
}

func (c *CheCluster) IsAirGapMode() bool {
	return c.Spec.Server.AirGapContainerRegistryHostname != "" ||
		c.Spec.Server.AirGapContainerRegistryOrganization != ""
}

func (c *CheCluster) IsImagePullerSpecEmpty() bool {
	return c.Spec.ImagePuller.Spec == (chev1alpha1.KubernetesImagePullerSpec{})
}

func (c *CheCluster) IsImagePullerImagesEmpty() bool {
	return len(c.Spec.ImagePuller.Spec.Images) == 0
}

func (c *CheCluster) GetCheHost() string {
	return strings.TrimPrefix(c.Status.CheURL, "https://")
}
