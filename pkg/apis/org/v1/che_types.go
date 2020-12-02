//
// Copyright (c) 2012-2019 Red Hat, Inc.
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

// Important: You should regenerate some generated code after modifying this file. At the root of the project:
// - Run "operator-sdk generate k8s": this will perform required changes in the "pkg/apis/org/v1/zz_generatedxxx" files
// - Run "operator-sdk generate openapi": this will generate the "deploy/crds/org_v1_checluster_crd.yaml" file
// - In the updated "deploy/crds/org_v1_checluster_crd.yaml": Delete all the `required:` openAPI rules in the CRD OpenApi schema.
// - Rename the new "deploy/crds/org_v1_checluster_crd.yaml" to "deploy/crds/org_v1_che_crd.yaml" to override it.
// IMPORTANT These 2 last steps are important to ensure backward compatibility with already existing `CheCluster` CRs that were created when no schema was provided.

import (
	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// +k8s:openapi-gen=true
// Desired configuration of the Che installation.
// Based on these settings, the operator automatically creates and maintains
// several config maps that will contain the appropriate environment variables
// the various components of the Che installation.
// These generated config maps should NOT be updated manually.
type CheClusterSpec struct {
	// General configuration settings related to the Che server
	// and the plugin and devfile registries
	// +optional
	Server CheClusterSpecServer `json:"server"`
	// Configuration settings related to the database used by the Che installation.
	// +optional
	Database CheClusterSpecDB `json:"database"`
	// Configuration settings related to the Authentication used by the Che installation.
	// +optional
	Auth CheClusterSpecAuth `json:"auth"`
	// Configuration settings related to the persistent storage used by the Che installation.
	// +optional
	Storage CheClusterSpecStorage `json:"storage"`
	// Configuration settings related to the metrics collection used by the Che installation.
	// +optional
	Metrics CheClusterSpecMetrics `json:"metrics"`
	// Configuration settings specific to Che installations made on upstream Kubernetes.
	// +optional
	K8s CheClusterSpecK8SOnly `json:"k8s"`
	// Kubernetes Image Puller configuration
	// +optional
	ImagePuller CheClusterSpecImagePuller `json:"imagePuller"`
}

// +k8s:openapi-gen=true
// General configuration settings related to the Che server
// and the plugin and devfile registries.
type CheClusterSpecServer struct {
	// Optional hostname (or url) to an alternate container registry to pull images from.
	// This value overrides the container registry hostname defined in all the default container images
	// involved in a Che deployment.
	// This is particularly useful to install Che in an air-gapped environment.
	// +optional
	AirGapContainerRegistryHostname string `json:"airGapContainerRegistryHostname,omitempty"`
	// Optional repository name of an alternate container registry to pull images from.
	// This value overrides the container registry organization defined in all the default container images
	// involved in a Che deployment.
	// This is particularly useful to install Che in an air-gapped environment.
	// +optional
	AirGapContainerRegistryOrganization string `json:"airGapContainerRegistryOrganization,omitempty"`
	// Overrides the container image used in Che deployment. This does NOT include the container image tag.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// +optional
	CheImage string `json:"cheImage,omitempty"`
	// Overrides the tag of the container image used in Che deployment.
	// Omit it or leave it empty to use the defaut image tag provided by the operator.
	// +optional
	CheImageTag string `json:"cheImageTag,omitempty"`
	// Overrides the image pull policy used in Che deployment.
	// Default value is `Always` for `nightly` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	CheImagePullPolicy corev1.PullPolicy `json:"cheImagePullPolicy,omitempty"`
	// Flavor of the installation. This is either `che` for upstream Che installations, or `codeready` for CodeReady Workspaces installation.
	// In most cases the default value should not be overridden.
	// +optional
	CheFlavor string `json:"cheFlavor,omitempty"`
	// Public hostname of the installed Che server.
	// If value is omitted then it will be automatically set by the operator.
	// (see the `cheHostTLSSecret` field).
	// +optional
	CheHost string `json:"cheHost,omitempty"`
	// Name of a secret containing certificates to secure ingress/route for the custom hostname of the installed Che server.
	// (see the `cheHost` field).
	// +optional
	CheHostTLSSecret string `json:"cheHostTLSSecret,omitempty"`
	// Log level for the Che server: `INFO` or `DEBUG`. Defaults to `INFO`.
	// +optional
	CheLogLevel string `json:"cheLogLevel,omitempty"`
	// Enables the debug mode for Che server. Defaults to `false`.
	// +optional
	CheDebug string `json:"cheDebug,omitempty"`
	// Comma-separated list of ClusterRoles that will be assigned to che ServiceAccount.
	// Be aware that che-operator has to already have all permissions in these ClusterRoles to be able to grant them.
	// +optional
	CheClusterRoles string `json:"cheClusterRoles,omitempty"`
	// Custom cluster role bound to the user for the Che workspaces.
	// The default roles are used if this is omitted or left blank.
	// +optional
	CheWorkspaceClusterRole string `json:"cheWorkspaceClusterRole,omitempty"`
	// Defines Kubernetes default namespace in which user's workspaces are created
	// if user does not override it.
	// It's possible to use <username>, <userid> and <workspaceid> placeholders (e.g.: che-workspace-<username>).
	// In that case, new namespace will be created for each user (or workspace).
	// Is used by OpenShift infra as well to specify Project
	// +optional
	WorkspaceNamespaceDefault string `json:"workspaceNamespaceDefault,omitempty"`
	// Defines if a user is able to specify Kubernetes namespace (or OpenShift project) different from the default.
	// It's NOT RECOMMENDED to configured true without OAuth configured. This property is also used by the OpenShift infra.
	// +optional
	AllowUserDefinedWorkspaceNamespaces bool `json:"allowUserDefinedWorkspaceNamespaces"`
	// Deprecated. The value of this flag is ignored.
	// Che operator will automatically detect if router certificate is self-signed.
	// If so it will be propagated to Che server and some other components.
	// +optional
	SelfSignedCert bool `json:"selfSignedCert"`
	// Name of the config-map with public certificates
	// to add to Java trust store of the Che server.
	// This is usually required when adding the OpenShift OAuth provider
	// which has https endpoint signed with self-signed cert. So,
	// Che server must be aware of its CA cert to be able to request it.
	// This is disabled by default.
	// +optional
	ServerTrustStoreConfigMapName string `json:"serverTrustStoreConfigMapName,omitempty"`
	// If enabled, then the certificate from `che-git-self-signed-cert`
	// config map will be propagated to the Che components and provide particular
	// configuration for Git.
	// +optional
	GitSelfSignedCert bool `json:"gitSelfSignedCert"`
	// Deprecated.
	// Instructs the operator to deploy Che in TLS mode.
	// This is enabled by default.
	// Disabling TLS may cause malfunction of some Che components.
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.displayName="Tls support"
	// +operator-sdk:gen-csv:customresourcedefinitions.specDescriptors.x-descriptors="urn:alm:descriptor:com.tectonic.ui:booleanSwitch"
	TlsSupport bool `json:"tlsSupport"`

	// Use internal cluster svc names to communicate between components to speed up the traffic
	// and avoid proxy issues.
	// The default value is `true`.
	UseInternalClusterSVCNames bool `json:"useInternalClusterSVCNames"`

	// Public URL of the Devfile registry, that serves sample, ready-to-use devfiles.
	// You should set it ONLY if you use an external devfile registry (see the `externalDevfileRegistry` field).
	// By default this will be automatically calculated by the operator.
	// +optional
	DevfileRegistryUrl string `json:"devfileRegistryUrl,omitempty"`
	// Overrides the container image used in the Devfile registry deployment. This includes the image tag.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// +optional
	DevfileRegistryImage string `json:"devfileRegistryImage,omitempty"`
	// Overrides the image pull policy used in the Devfile registry deployment.
	// Default value is `Always` for `nightly` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	DevfileRegistryPullPolicy corev1.PullPolicy `json:"devfileRegistryPullPolicy,omitempty"`
	// Overrides the memory limit used in the Devfile registry deployment. Defaults to 256Mi.
	// +optional
	DevfileRegistryMemoryLimit string `json:"devfileRegistryMemoryLimit,omitempty"`
	// Overrides the memory request used in the Devfile registry deployment. Defaults to 16Mi.
	// +optional
	DevfileRegistryMemoryRequest string `json:"devfileRegistryMemoryRequest,omitempty"`
	// Devfile registry ingress custom settings
	// +optional
	DevfileRegistryIngress IngressCustomSettings `json:"devfileRegistryIngress,omitempty"`
	// Devfile registry route custom settings
	// +optional
	DevfileRegistryRoute RouteCustomSettings `json:"devfileRegistryRoute,omitempty"`
	// Instructs the operator on whether or not to deploy a dedicated Devfile registry server.
	// By default a dedicated devfile registry server is started.
	// But if `externalDevfileRegistry` is `true`, then no such dedicated server will be started by the operator
	// and you will have to manually set the `devfileRegistryUrl` field
	// +optional
	ExternalDevfileRegistry bool `json:"externalDevfileRegistry"`
	// Public URL of the Plugin registry, that serves sample ready-to-use devfiles.
	// You should set it ONLY if you use an external devfile registry (see the `externalPluginRegistry` field).
	// By default this will be automatically calculated by the operator.
	// +optional
	PluginRegistryUrl string `json:"pluginRegistryUrl,omitempty"`
	// Overrides the container image used in the Plugin registry deployment. This includes the image tag.
	// Omit it or leave it empty to use the default container image provided by the operator.
	// +optional
	PluginRegistryImage string `json:"pluginRegistryImage,omitempty"`
	// Overrides the image pull policy used in the Plugin registry deployment.
	// Default value is `Always` for `nightly` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	PluginRegistryPullPolicy corev1.PullPolicy `json:"pluginRegistryPullPolicy,omitempty"`
	// Overrides the memory limit used in the Plugin registry deployment. Defaults to 256Mi.
	// +optional
	PluginRegistryMemoryLimit string `json:"pluginRegistryMemoryLimit,omitempty"`
	// Overrides the memory request used in the Plugin registry deployment. Defaults to 16Mi.
	// +optional
	PluginRegistryMemoryRequest string `json:"pluginRegistryMemoryRequest,omitempty"`
	// Plugin registry ingress custom settings
	// +optional
	PluginRegistryIngress IngressCustomSettings `json:"pluginRegistryIngress,omitempty"`
	// Plugin registry route custom settings
	// +optional
	PluginRegistryRoute RouteCustomSettings `json:"pluginRegistryRoute,omitempty"`
	// Instructs the operator on whether or not to deploy a dedicated Plugin registry server.
	// By default a dedicated plugin registry server is started.
	// But if `externalPluginRegistry` is `true`, then no such dedicated server will be started by the operator
	// and you will have to manually set the `pluginRegistryUrl` field.
	// +optional
	ExternalPluginRegistry bool `json:"externalPluginRegistry"`
	// Map of additional environment variables that will be applied in the generated `che` config map to be used by the Che server,
	// in addition to the values already generated from other fields of the `CheCluster` custom resource (CR).
	// If `customCheProperties` contains a property that would be normally generated in `che` config map from other
	// CR fields, then the value defined in the `customCheProperties` will be used instead.
	// +optional
	CustomCheProperties map[string]string `json:"customCheProperties,omitempty"`
	// URL (protocol+hostname) of the proxy server.
	// This drives the appropriate changes in the `JAVA_OPTS` and `https(s)_proxy` variables
	// in the Che server and workspaces containers. Only use when configuring a proxy is required.
	// Operator respects OpenShift cluster wide proxy configuration and no additional configuration is required,
	// but defining `proxyUrl` in a custom resource leads to overrides the cluster proxy configuration with
	// fields `proxyUrl`, `proxyPort`, `proxyUser` and `proxyPassword` from the custom resource.
	// (see the doc https://docs.openshift.com/container-platform/4.4/networking/enable-cluster-wide-proxy.html)
	// (see also the `proxyPort` and `nonProxyHosts` fields).
	// +optional
	ProxyURL string `json:"proxyURL,omitempty"`
	// Port of the proxy server. Only use when configuring a proxy is required.
	// (see also the `proxyURL` and `nonProxyHosts` fields).
	// +optional
	ProxyPort string `json:"proxyPort,omitempty"`
	// List of hosts that should not use the configured proxy.
	// So specify wild card domain use the following form `.<DOMAIN>` and `|` as delimiter, eg: `localhost|.my.host.com|123.42.12.32`
	// Only use when configuring a proxy is required.
	// Operator respects OpenShift cluster wide proxy configuration and no additional configuration is required,
	// but defining `nonProxyHosts` in a custom resource leads to merging non proxy hosts lists from the
	// cluster proxy configuration and ones defined in the custom resources.
	// (see the doc https://docs.openshift.com/container-platform/4.4/networking/enable-cluster-wide-proxy.html)
	// (see also the `proxyURL` fields).
	NonProxyHosts string `json:"nonProxyHosts,omitempty"`
	// User name of the proxy server.
	// Only use when configuring a proxy is required
	// (see also the `proxyURL`, `proxyPassword` and `proxySecret` fields).
	// +optional
	ProxyUser string `json:"proxyUser,omitempty"`
	// Password of the proxy server
	// Only use when proxy configuration is required
	// (see also the `proxyURL`, `proxyUser` and `proxySecret` fields).
	// +optional
	ProxyPassword string `json:"proxyPassword,omitempty"`
	// The secret that contains `user` and `password` for a proxy server.
	// If the secret is defined then `proxyUser` and `proxyPassword` are ignored
	// +optional
	ProxySecret string `json:"proxySecret,omitempty"`
	// Overrides the memory request used in the Che server deployment. Defaults to 512Mi.
	// +optional
	ServerMemoryRequest string `json:"serverMemoryRequest,omitempty"`
	// Overrides the memory limit used in the Che server deployment. Defaults to 1Gi.
	// +optional
	ServerMemoryLimit string `json:"serverMemoryLimit,omitempty"`

	// Sets the server and workspaces exposure type. Possible values are "multi-host", "single-host", "default-host".
	// Defaults to "multi-host" which creates a separate ingress (or route on OpenShift) for every required
	// endpoint.
	// "single-host" makes Che exposed on a single hostname with workspaces exposed on subpaths. Please read the docs
	// to learn about the limitations of this approach. Also consult the `singleHostExposureType` property to further configure
	// how the operator and Che server make that happen on Kubernetes.
	// "default-host" exposes che server on the host of the cluster. Please read the docs to learn about
	// the limitations of this approach.
	// +optional
	ServerExposureStrategy string `json:"serverExposureStrategy,omitempty"`

	// The image used for the gateway in the single host mode.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// +optional
	SingleHostGatewayImage string `json:"singleHostGatewayImage,omitempty"`

	// The image used for the gateway sidecar that provides configuration to the gateway.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// +optional
	SingleHostGatewayConfigSidecarImage string `json:"singleHostGatewayConfigSidecarImage,omitempty"`

	// The labels that need to be present (and are put) on the configmaps representing the gateway configuration.
	// +optional
	SingleHostGatewayConfigMapLabels labels.Set `json:"singleHostGatewayConfigMapLabels,omitempty"`
	// Che server ingress custom settings
	// +optional
	CheServerIngress IngressCustomSettings `json:"cheServerIngress,omitempty"`
	// Che server route custom settings
	// +optional
	CheServerRoute RouteCustomSettings `json:"cheServerRoute,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the database used by the Che installation.
type CheClusterSpecDB struct {
	// Instructs the operator on whether or not to deploy a dedicated database.
	// By default a dedicated Postgres database is deployed as part of the Che installation.
	// But if `externalDb` is `true`, then no dedicated database will be deployed by the operator
	// and you might need to provide connection details to the external DB you want to use.
	// See also all the fields starting with: `chePostgres`.
	// +optional
	ExternalDb bool `json:"externalDb"`
	// Postgres Database hostname that the Che server uses to connect to. Defaults to postgres.
	// This value should be overridden ONLY when using an external database (see field `externalDb`).
	// In the default case it will be automatically set by the operator.
	// +optional
	ChePostgresHostName string `json:"chePostgresHostName,omitempty"`
	// Postgres Database port that the Che server uses to connect to. Defaults to 5432.
	// This value should be overridden ONLY when using an external database (see field `externalDb`).
	// In the default case it will be automatically set by the operator.
	// +optional
	ChePostgresPort string `json:"chePostgresPort,omitempty"`
	// Postgres user that the Che server should use to connect to the DB. Defaults to `pgche`.
	// +optional
	ChePostgresUser string `json:"chePostgresUser,omitempty"`
	// Postgres password that the Che server should use to connect to the DB.
	// If omitted or left blank, it will be set to an auto-generated value.
	// +optional
	ChePostgresPassword string `json:"chePostgresPassword,omitempty"`
	// Postgres database name that the Che server uses to connect to the DB. Defaults to `dbche`.
	// +optional
	ChePostgresDb string `json:"chePostgresDb,omitempty"`
	// The secret that contains Postgres `user` and `password` that the Che server should use to connect to the DB.
	// If the secret is defined then `chePostgresUser` and `chePostgresPassword` are ignored.
	// If the value is omitted or left blank then there are two scenarios:
	// 1. `chePostgresUser` and `chePostgresPassword` are defined, then they will be used to connect to the DB.
	// 2. `chePostgresUser` or `chePostgresPassword` are not defined, then a new secret with the name `che-postgres-secret`
	// will be created with default value of `pgche` for `user` and with an auto-generated value for `password`.
	// +optional
	ChePostgresSecret string `json:"chePostgresSecret,omitempty"`
	// Overrides the container image used in the Postgres database deployment. This includes the image tag.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// +optional
	PostgresImage string `json:"postgresImage,omitempty"`
	// Overrides the image pull policy used in the Postgres database deployment.
	// Default value is `Always` for `nightly` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	PostgresImagePullPolicy corev1.PullPolicy `json:"postgresImagePullPolicy,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the Authentication used by the Che installation.
type CheClusterSpecAuth struct {
	// Instructs the operator on whether or not to deploy a dedicated Identity Provider (Keycloak or RH SSO instance).
	// By default a dedicated Identity Provider server is deployed as part of the Che installation.
	// But if `externalIdentityProvider` is `true`, then no dedicated identity provider will be deployed by the operator
	// and you might need to provide details about the external identity provider you want to use.
	// See also all the other fields starting with: `identityProvider`.
	// +optional
	ExternalIdentityProvider bool `json:"externalIdentityProvider"`
	// Public URL of the Identity Provider server (Keycloak / RH SSO server).
	// You should set it ONLY if you use an external Identity Provider (see the `externalIdentityProvider` field).
	// By default this will be automatically calculated and set by the operator.
	// +optional
	IdentityProviderURL string `json:"identityProviderURL,omitempty"`
	// Overrides the name of the Identity Provider admin user. Defaults to `admin`.
	// +optional
	IdentityProviderAdminUserName string `json:"identityProviderAdminUserName,omitempty"`
	// Overrides the password of Keycloak admin user.
	// This is useful to override it ONLY if you use an external Identity Provider (see the `externalIdentityProvider` field).
	// If omitted or left blank, it will be set to an auto-generated password.
	// +optional
	IdentityProviderPassword string `json:"identityProviderPassword,omitempty"`
	// The secret that contains `user` and `password` for Identity Provider.
	// If the secret is defined then `identityProviderAdminUserName` and `identityProviderPassword` are ignored.
	// If the value is omitted or left blank then there are two scenarios:
	// 1. `identityProviderAdminUserName` and `identityProviderPassword` are defined, then they will be used.
	// 2. `identityProviderAdminUserName` or `identityProviderPassword` are not defined, then a new secret
	// with the name `che-identity-secret` will be created with default value `admin` for `user` and
	// with an auto-generated value for `password`.
	// +optional
	IdentityProviderSecret string `json:"identityProviderSecret,omitempty"`
	// Name of a Identity provider (Keycloak / RH SSO) realm that should be used for Che.
	// This is useful to override it ONLY if you use an external Identity Provider (see the `externalIdentityProvider` field).
	// If omitted or left blank, it will be set to the value of the `flavour` field.
	// +optional
	IdentityProviderRealm string `json:"identityProviderRealm,omitempty"`
	// Name of a Identity provider (Keycloak / RH SSO) `client-id` that should be used for Che.
	// This is useful to override it ONLY if you use an external Identity Provider (see the `externalIdentityProvider` field).
	// If omitted or left blank, it will be set to the value of the `flavour` field suffixed with `-public`.
	// +optional
	IdentityProviderClientId string `json:"identityProviderClientId,omitempty"`
	// Password for The Identity Provider (Keycloak / RH SSO) to connect to the database.
	// This is useful to override it ONLY if you use an external Identity Provider (see the `externalIdentityProvider` field).
	// If omitted or left blank, it will be set to an auto-generated password.
	// +optional
	IdentityProviderPostgresPassword string `json:"identityProviderPostgresPassword,omitempty"`
	// The secret that contains `password` for The Identity Provider (Keycloak / RH SSO) to connect to the database.
	// If the secret is defined then `identityProviderPostgresPassword` will be ignored.
	// If the value is omitted or left blank then there are two scenarios:
	// 1. `identityProviderPostgresPassword` is defined, then it will be used to connect to the database.
	// 2. `identityProviderPostgresPassword` is not defined, then a new secret with the name `che-identity-postgres-secret`
	// will be created with an auto-generated value for `password`.
	// +optional
	IdentityProviderPostgresSecret string `json:"identityProviderPostgresSecret,omitempty"`
	// Forces the default `admin` Che user to update password on first login. Defaults to `false`.
	// +optional
	UpdateAdminPassword bool `json:"updateAdminPassword"`
	// Enables the integration of the identity provider (Keycloak / RHSSO) with OpenShift OAuth. Enabled by default on OpenShift.
	// This will allow users to directly login with their Openshift user through the Openshift login,
	// and have their workspaces created under personal OpenShift namespaces.
	// WARNING: the `kubeadmin` user is NOT supported, and logging through it will NOT allow accessing the Che Dashboard.
	// +optional
	OpenShiftoAuth bool `json:"openShiftoAuth"`
	// Name of the OpenShift `OAuthClient` resource used to setup identity federation on the OpenShift side. Auto-generated if left blank.
	// See also the `OpenShiftoAuth` field.
	// +optional
	OAuthClientName string `json:"oAuthClientName,omitempty"`
	// Name of the secret set in the OpenShift `OAuthClient` resource used to setup identity federation on the OpenShift side. Auto-generated if left blank.
	// See also the `OAuthClientName` field.
	// +optional
	OAuthSecret string `json:"oAuthSecret,omitempty"`
	// Overrides the container image used in the Identity Provider (Keycloak / RH SSO) deployment. This includes the image tag.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// +optional
	IdentityProviderImage string `json:"identityProviderImage,omitempty"`
	// Overrides the image pull policy used in the Identity Provider (Keycloak / RH SSO) deployment.
	// Default value is `Always` for `nightly` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	IdentityProviderImagePullPolicy corev1.PullPolicy `json:"identityProviderImagePullPolicy,omitempty"`
	// Ingress custom settings
	// +optional
	IdentityProviderIngress IngressCustomSettings `json:"identityProviderIngress,omitempty"`
	// Route custom settings
	// +optional
	IdentityProviderRoute RouteCustomSettings `json:"identityProviderRoute,omitempty"`
}

// Ingress custom settings, can be extended in the future
type IngressCustomSettings struct {
	// Comma separated list of labels that can be used to organize and categorize (scope and select) objects.
	// +optional
	Labels string `json:"labels,omitempty"`
}

// Route custom settings, can be extended in the future
type RouteCustomSettings struct {
	// Comma separated list of labels that can be used to organize and categorize (scope and select) objects.
	// +optional
	Labels string `json:"labels,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings related to the persistent storage used by the Che installation.
type CheClusterSpecStorage struct {
	// Persistent volume claim strategy for the Che server.
	// This Can be:`common` (all workspaces PVCs in one volume),
	// `per-workspace` (one PVC per workspace for all declared volumes) and `unique` (one PVC per declared volume).
	// Defaults to `common`.
	// +optional
	PvcStrategy string `json:"pvcStrategy,omitempty"`
	// Size of the persistent volume claim for workspaces. Defaults to `1Gi`
	// +optional
	PvcClaimSize string `json:"pvcClaimSize,omitempty"`
	// Instructs the Che server to launch a special pod to pre-create a subpath in the Persistent Volumes.
	// Defaults to `false`, however it might need to enable it according to the configuration of your K8S cluster.
	// +optional
	PreCreateSubPaths bool `json:"preCreateSubPaths"`
	// Overrides the container image used to create sub-paths in the Persistent Volumes. This includes the image tag.
	// Omit it or leave it empty to use the defaut container image provided by the operator.
	// See also the `preCreateSubPaths` field.
	// +optional
	PvcJobsImage string `json:"pvcJobsImage,omitempty"`
	// Storage class for the Persistent Volume Claim dedicated to the Postgres database.
	// If omitted or left blank, default storage class is used.
	// +optional
	PostgresPVCStorageClassName string `json:"postgresPVCStorageClassName,omitempty"`
	// Storage class for the Persistent Volume Claims dedicated to the Che workspaces.
	// If omitted or left blank, default storage class is used.
	// +optional
	WorkspacePVCStorageClassName string `json:"workspacePVCStorageClassName,omitempty"`
}

// +k8s:openapi-gen=true
// Configuration settings specific to Che installations made on upstream Kubernetes.
type CheClusterSpecK8SOnly struct {
	// Global ingress domain for a K8S cluster. This MUST be explicitly specified: there are no defaults.
	IngressDomain string `json:"ingressDomain,omitempty"`
	// Strategy for ingress creation. This can be `multi-host` (host is explicitly provided in ingress),
	// `single-host` (host is provided, path-based rules) and `default-host.*`(no host is provided, path-based rules).
	// Defaults to `"multi-host`
	// Deprecated in favor of "serverExposureStrategy" in the "server" section, which defines this regardless of the cluster type.
	// If both are defined, `serverExposureStrategy` takes precedence.
	// +optional
	IngressStrategy string `json:"ingressStrategy,omitempty"`
	// Ingress class that will define the which controler will manage ingresses. Defaults to `nginx`.
	// NB: This drives the `is kubernetes.io/ingress.class` annotation on Che-related ingresses.
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`
	// Name of a secret that will be used to setup ingress TLS termination if TLS is enabled.
	// If the field is empty string, then default cluster certificate will be used.
	// See also the `tlsSupport` field.
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
	// FSGroup the Che pod and Workspace pods containers should run in. Defaults to `1724`.
	// +optional
	SecurityContextFsGroup string `json:"securityContextFsGroup,omitempty"`
	// ID of the user the Che pod and Workspace pods containers should run as. Default to `1724`.
	// +optional
	SecurityContextRunAsUser string `json:"securityContextRunAsUser,omitempty"`
	// When the serverExposureStrategy is set to "single-host", the way the server, registries and workspaces
	// are exposed is further configured by this property. The possible values are "native" (which means
	// that the server and workspaces are exposed using ingresses on K8s) or "gateway" where the server
	// and workspaces are exposed using a custom gateway based on Traefik. All the endpoints whether backed by the ingress
	// or gateway "route" always point to the subpaths on the same domain.
	// Defaults to "native".
	// +optional
	SingleHostExposureType string `json:"singleHostExposureType,omitempty"`
}

type CheClusterSpecMetrics struct {
	// Enables `metrics` Che server endpoint. Default to `true`.
	// +optional
	Enable bool `json:"enable"`
}

// +k8s:openapi-gen=true
// Configuration settings for installation and configuration of the Kubernetes Image Puller
// See https://github.com/che-incubator/kubernetes-image-puller-operator
type CheClusterSpecImagePuller struct {
	// Install and configure the Kubernetes Image Puller Operator. If true and no spec is provided,
	// it will create a default KubernetesImagePuller object to be managed by the Operator.
	// If false, the KubernetesImagePuller object will be deleted, and the operator will be uninstalled,
	// regardless of whether or not a spec is provided.
	Enable bool `json:"enable"`
	// A KubernetesImagePullerSpec to configure the image puller in the CheCluster
	// +optional
	Spec chev1alpha1.KubernetesImagePullerSpec `json:"spec"`
}

// CheClusterStatus defines the observed state of Che installation
type CheClusterStatus struct {
	// Indicates if or not a Postgres instance has been correctly provisioned
	// +optional
	DbProvisoned bool `json:"dbProvisioned"`
	// Indicates whether an Identity Provider instance (Keycloak / RH SSO) has been provisioned with realm, client and user
	// +optional
	KeycloakProvisoned bool `json:"keycloakProvisioned"`
	// Indicates whether an Identity Provider instance (Keycloak / RH SSO) has been configured to integrate with the OpenShift OAuth.
	// +optional
	OpenShiftoAuthProvisioned bool `json:"openShiftoAuthProvisioned"`
	// Status of a Che installation. Can be `Available`, `Unavailable`, or `Available, Rolling Update in Progress`
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Status"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:io.kubernetes.phase"
	CheClusterRunning string `json:"cheClusterRunning"`
	// Current installed Che version
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="displayName: Eclipse Che version"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:org.w3:link"
	CheVersion string `json:"cheVersion"`
	// Public URL to the Che server
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Eclipse Che URL"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:org.w3:link"
	CheURL string `json:"cheURL"`
	// Public URL to the Identity Provider server (Keycloak / RH SSO).
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Keycloak Admin Console URL"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:org.w3:link"
	KeycloakURL string `json:"keycloakURL"`
	// Public URL to the Devfile registry
	// +optional
	DevfileRegistryURL string `json:"devfileRegistryURL"`
	// Public URL to the Plugin registry
	// +optional
	PluginRegistryURL string `json:"pluginRegistryURL"`
	// A human readable message indicating details about why the pod is in this condition.
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Message"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:text"
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Reason"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:text"
	Reason string `json:"reason,omitempty"`
	// A URL that can point to some URL where to find help related to the current Operator status.
	// +optional
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors=true
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.displayName="Help link"
	// +operator-sdk:gen-csv:customresourcedefinitions.statusDescriptors.x-descriptors="urn:alm:descriptor:org.w3:link"
	HelpLink string `json:"helpLink,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// The `CheCluster` custom resource allows defining and managing a Che server installation
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +operator-sdk:gen-csv:customresourcedefinitions.displayName="Eclipse Che Cluster"
type CheCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Desired configuration of the Che installation.
	// Based on these settings, the operator automatically creates and maintains
	// several config maps that will contain the appropriate environment variables
	// the various components of the Che installation.
	// These generated config maps should NOT be updated manually.
	Spec CheClusterSpec `json:"spec,omitempty"`

	// CheClusterStatus defines the observed state of Che installation
	Status CheClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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
