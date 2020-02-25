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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	// In most cases the default value should not be overriden.
	// +optional
	CheFlavor string `json:"cheFlavor,omitempty"`
	// Public hostname of the installed Che server. This will be automatically set by the operator.
	// In most cases the default value set by the operator should not be overriden.
	// +optional
	CheHost string `json:"cheHost,omitempty"`
	// Log level for the Che server: `INFO` or `DEBUG`. Defaults to `INFO`.
	// +optional
	CheLogLevel string `json:"cheLogLevel,omitempty"`
	// Enables the debug mode for Che server. Defaults to `false`.
	// +optional
	CheDebug string `json:"cheDebug,omitempty"`
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
	// Enables the support of OpenShift clusters whose router uses self-signed certificates.
	// When enabled, the operator retrieves the default self-signed certificate of OpenShift routes
	// and adds it to the Java trust store of the Che server.
	// This is usually required when activating the `tlsSupport` field on demo OpenShift clusters
	// that have not been setup with a valid certificate for the routes.
	// This is disabled by default.
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
	// Instructs the operator to deploy Che in TLS mode, ie with TLS routes or ingresses.
	// This is disabled by default.
	// WARNING: Enabling TLS might require enabling the `selfSignedCert` field also in some cases.
	// +optional
	TlsSupport bool `json:"tlsSupport"`
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
	// Omit it or leave it empty to use the defaut container image provided by the operator.
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
	// in the Che server and workspaces containers.
	// Only use when configuring a proxy is required.
	// +optional
	ProxyURL string `json:"proxyURL,omitempty"`
	// Port of the proxy server.
	// Only use when configuring a proxy is required
	// (see also the `proxyURL` field).
	// +optional
	ProxyPort string `json:"proxyPort,omitempty"`
	// List of hosts that should not use the configured proxy. Use `|`` as delimiter, eg `localhost|my.host.com|123.42.12.32`
	// Only use when configuring a proxy is required
	// (see also the `proxyURL` field).
	NonProxyHosts string `json:"nonProxyHosts,omitempty"`
	// User name of the proxy server.
	// Only use when configuring a proxy is required
	// (see also the `proxyURL` field).
	// +optional
	ProxyUser string `json:"proxyUser,omitempty"`
	// Password of the proxy server
	//
	// Only use when proxy configuration is required
	// (see also the `proxyUser` field).
	// +optional
	ProxyPassword string `json:"proxyPassword,omitempty"`
	// Overrides the memory request used in the Che server deployment. Defaults to 512Mi.
	// +optional
	ServerMemoryRequest string `json:"serverMemoryRequest,omitempty"`
	// Overrides the memory limit used in the Che server deployment. Defaults to 1Gi.
	// +optional
	ServerMemoryLimit string `json:"serverMemoryLimit,omitempty"`
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
	// Forces the default `admin` Che user to update password on first login. Defaults to `false`.
	// +optional
	UpdateAdminPassword bool `json:"updateAdminPassword"`
	// Enables the integration of the identity provider (Keycloak / RHSSO) with OpenShift OAuth. Enabled by defaumt on OpenShift.
	// This will allow users to directly login with their Openshift user throug the Openshift login,
	// and have their workspaces created under personnal OpenShift namespaces.
	// WARNING: the `kuebadmin` user is NOT supported, and logging through it will NOT allow accessing the Che Dashboard.
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
	// +optional
	IngressStrategy string `json:"ingressStrategy,omitempty"`
	// Ingress class that will define the which controler will manage ingresses. Defaults to `nginx`.
	// NB: This drives the `is kubernetes.io/ingress.class` annotation on Che-related ingresses.
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`
	// Name of a secret that will be used to setup ingress TLS termination if TLS is enabled.
	// See also the `tlsSupport` field.
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
	// FSGroup the Che pod and Workspace pods containers should run in. Defaults to `1724`.
	// +optional
	SecurityContextFsGroup string `json:"securityContextFsGroup,omitempty"`
	// ID of the user the Che pod and Workspace pods containers should run as. Default to `1724`.
	// +optional
	SecurityContextRunAsUser string `json:"securityContextRunAsUser,omitempty"`
}

type CheClusterSpecMetrics struct {
	// Enables `metrics` Che server endpoint. Default to `false`.
	// +optional
	Enable bool `json:"enable"`
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
	CheClusterRunning string `json:"cheClusterRunning"`
	// Current installed Che version
	// +optional
	CheVersion string `json:"cheVersion"`
	// Public URL to the Che server
	// +optional
	CheURL string `json:"cheURL"`
	// Public URL to the Identity Provider server (Keycloak / RH SSO).
	// +optional
	KeycloakURL string `json:"keycloakURL"`
	// Public URL to the Devfile registry
	// +optional
	DevfileRegistryURL string `json:"devfileRegistryURL"`
	// Public URL to the Plugin registry
	// +optional
	PluginRegistryURL string `json:"pluginRegistryURL"`
	// A human readable message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A URL that can point to some URL where to find help related to the current Operator status.
	// +optional
	HelpLink string `json:"helpLink,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// The `CheCluster` custom resource allows defining and managing a Che server installation
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type CheCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Desired configuration of the Che installation.
	// Based on these settings, the operator automatically creates and maintains
	// several config maps that will contain the appropriate environment variables
	// the various components of the Che installation.
	// These generated config maps should NOT be updated manually.
	Spec   CheClusterSpec   `json:"spec,omitempty"`
	
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
