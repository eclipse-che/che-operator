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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +k8s:openapi-gen=true
// CheClusterSpec defines the desired state of CheCluster
type CheClusterSpec struct {
	Server   CheClusterSpecServer  `json:"server"`
	Database CheClusterSpecDB      `json:"database"`
	Auth     CheClusterSpecAuth    `json:"auth"`
	Storage  CheClusterSpecStorage `json:"storage"`

	// +optional
	K8s CheClusterSpecK8SOnly `json:"k8s,omitempty"`
}

// +k8s:openapi-gen=true
type CheClusterSpecServer struct {
	// AirGapContainerRegistryHostname is the hostname to the internal registry to pull images from in the air-gapped environment
	// +optional
	AirGapContainerRegistryHostname string `json:"airGapContainerRegistryHostname,omitempty"`
	// AirGapContainerRegistryOrganization is the repository name in the registry to pull images from in the air-gapped environment
	// +optional
	AirGapContainerRegistryOrganization string `json:"airGapContainerRegistryOrganization,omitempty"`
	// CheImage is a server image used in Che deployment
	// +optional
	CheImage string `json:"cheImage,omitempty"`
	// CheImageTag is a tag of an image used in Che deployment
	// +optional
	CheImageTag string `json:"cheImageTag,omitempty"`
	// CheImagePullPolicy is the image pull policy used in Che registry deployment: default value is Always
	// +optional
	CheImagePullPolicy corev1.PullPolicy `json:"cheImagePullPolicy,omitempty"`
	// CheFlavor is an installation flavor. Can be 'che' - upstream or 'codeready' - CodeReady Workspaces. Defaults to 'che'
	// +optional
	CheFlavor string `json:"cheFlavor,omitempty"`
	// CheHost is an env consumer by server. Detected automatically from Che route
	// +optional
	CheHost string `json:"cheHost,omitempty"`
	// CheLostLevel is log level for Che server: INFO or DEBUG. Defaults to INFO
	// +optional
	CheLogLevel string `json:"cheLogLevel,omitempty"`
	// CheDebug is debug mode for Che server. Defaults to false
	// +optional
	CheDebug string `json:"cheDebug,omitempty"`
	// CustomClusterRoleName specifies a custom cluster role to user for the Che workspaces
	// The default roles are used if this is left blank.
	// +optional
	CheWorkspaceClusterRole string `json:"cheWorkspaceClusterRole,omitempty"`
	// SelfSignedCert signal about the necessity to get OpenShift router tls secret
	// and extract certificate to add it to Java trust store for Che server
	// +optional
	SelfSignedCert bool `json:"selfSignedCert,omitempty"`
	// TlsSupport instructs an operator to deploy Che in TLS mode, ie with TLS routes or ingresses
	// +optional
	TlsSupport bool `json:"tlsSupport,omitempty"`
	// DevfileRegistryUrl is an endpoint serving sample ready-to-use devfiles. Defaults to generated route
	// +optional
	DevfileRegistryUrl string `json:"devfileRegistryUrl,omitempty"`
	// DevfileRegistryImage is image:tag used in Devfile registry deployment
	// +optional
	DevfileRegistryImage string `json:"devfileRegistryImage,omitempty"`
	// DevfileRegistryImagePullPolicy is the image pull policy used in Devfile registry deployment
	// +optional
	DevfileRegistryPullPolicy corev1.PullPolicy `json:"devfileRegistryPullPolicy,omitempty"`
	// DevfileRegistryMemoryLimit is the memory limit used in Devfile registry deployment
	// +optional
	DevfileRegistryMemoryLimit string `json:"devfileRegistryMemoryLimit,omitempty"`
	// DevfileRegistryMemoryRequest is the memory request used in Devfile registry deployment
	// +optional
	DevfileRegistryMemoryRequest string `json:"devfileRegistryMemoryRequest,omitempty"`
	// ExternalDevfileRegistry instructs operator on whether or not to deploy a dedicated Devfile registry server
	// By default a dedicated devfile registry server is started.
	// But if ExternalDevfileRegistry is `true`, then no such dedicated server will be started by the operator
	// +optional
	ExternalDevfileRegistry bool `json:"externalDevfileRegistry,omitempty"`
	// PluginRegistryUrl is an endpoint serving plugin definitions. Defaults to generated route
	// +optional
	PluginRegistryUrl string `json:"pluginRegistryUrl,omitempty"`
	// PluginRegistryImage is image:tag used in Plugin registry deployment
	// +optional
	PluginRegistryImage string `json:"pluginRegistryImage,omitempty"`
	// PluginRegistryImagePullPolicy is the image pull policy used in Plugin registry deployment
	// +optional
	PluginRegistryPullPolicy corev1.PullPolicy `json:"pluginRegistryPullPolicy,omitempty"`
	// PluginRegistryMemoryLimit is the memory limit used in Plugin registry deployment
	// +optional
	PluginRegistryMemoryLimit string `json:"pluginRegistryMemoryLimit,omitempty"`
	// PluginRegistryMemoryRequest is the memory request used in Plugin registry deployment
	// +optional
	PluginRegistryMemoryRequest string `json:"pluginRegistryMemoryRequest,omitempty"`
	// ExternalPluginRegistry instructs operator on whether or not to deploy a dedicated Plugin registry server
	// By default a dedicated plugin registry server is started.
	// But if ExternalPluginRegistry is `true`, then no such dedicated server will be started by the operator
	// +optional
	ExternalPluginRegistry bool `json:"externalPluginRegistry,omitempty"`
	// CustomCheProperties is a list of additional environment variables that will be applied in the che config map,
	// in addition to the values already generated from other fields of the custom resource (CR).
	// If CustomCheProperties contains a property that would be normally generated in che config map from other
	// CR fields, then the value in the CustomCheProperties will be used.
	// +optional
	CustomCheProperties map[string]string `json:"customCheProperties,omitempty"`
	// ProxyURL is protocol+hostname of a proxy server. Automatically added as JAVA_OPTS and https(s)_proxy
	// to Che server and workspaces containers
	// +optional
	ProxyURL string `json:"proxyURL,omitempty"`
	// ProxyPort is port of a proxy server
	// +optional
	ProxyPort string `json:"proxyPort,omitempty"`
	// NonProxyHosts is a list of non-proxy hosts. Use | as delimiter, eg localhost|my.host.com|123.42.12.32
	// +optional
	NonProxyHosts string `json:"nonProxyHosts,omitempty"`
	// ProxyUser is username for a proxy server
	// +optional
	ProxyUser string `json:"proxyUser,omitempty"`
	// ProxyPassword is password for a proxy user
	// +optional
	ProxyPassword string `json:"proxyPassword,omitempty"`
	// ServerMemoryRequest sets mem request for server deployment. Defaults to 512Mi
	// +optional
	ServerMemoryRequest string `json:"serverMemoryRequest,omitempty"`
	// ServerMemoryLimit sets mem limit for server deployment. Defaults to 1Gi
	// +optional
	ServerMemoryLimit string `json:"serverMemoryLimit,omitempty"`
}

// +k8s:openapi-gen=true
type CheClusterSpecDB struct {
	// ExternalDB instructs the operator either to skip deploying Postgres,
	// and passes connection details of existing DB to Che server (when set to true)
	// or a new Postgres deployment is created
	// +optional
	ExternalDb bool `json:"externalDb,omitempty"`
	// ChePostgresDBHostname is Postgres Database hostname that Che server uses to connect to. Defaults to postgres
	// +optional
	ChePostgresHostName string `json:"chePostgresHostName,omitempty"`
	// ChePostgresPort is Postgres Database port that Che server uses to connect to. Defaults to 5432
	// +optional
	ChePostgresPort string `json:"chePostgresPort,omitempty"`
	// ChePostgresUser is Postgres user that Che server when making a db connection. Defaults to pgche
	// +optional
	ChePostgresUser string `json:"chePostgresUser,omitempty"`
	// ChePostgresPassword is password of a postgres user. Auto-generated when left blank
	// +optional
	ChePostgresPassword string `json:"chePostgresPassword,omitempty"`
	// ChePostgresDb is Postgres database name that Che server uses to connect to. Defaults to dbche
	// +optional
	ChePostgresDb string `json:"chePostgresDb,omitempty"`
	// PostgresImage is an image used in Postgres deployment in format image:tag. Defaults to registry.redhat.io/rhscl/postgresql-96-rhel7 (see pkg/deploy/defaults.go for latest tag)
	// +optional
	PostgresImage string `json:"postgresImage,omitempty"`
	// PostgresImagePullPolicy is the image pull policy used in Postgres registry deployment: default value is Always
	// +optional
	PostgresImagePullPolicy corev1.PullPolicy `json:"postgresImagePullPolicy,omitempty"`
}

// +k8s:openapi-gen=true
type CheClusterSpecAuth struct {
	// ExternalIdentityProvider instructs operator on whether or not to deploy Keycloak/RH SSO instance. When set to true provision connection details
	// +optional
	ExternalIdentityProvider bool `json:"externalIdentityProvider,omitempty"`
	// IdentityProviderURL is retrieved from respective route/ingress unless explicitly specified in CR (when externalIdentityProvider is true)
	// +optional
	IdentityProviderURL string `json:"identityProviderURL,omitempty"`
	// IdentityProviderURL is retrieved from respective route/ingress unless explicitly specified in CR (when externalIdentityProvider is true)
	//IdentityProviderURL string `json:"identityProviderURL"`
	// IdentityProviderAdminUserName is a desired admin username of Keycloak admin user (applicable only when externalIdentityProvider is false)
	// +optional
	IdentityProviderAdminUserName string `json:"identityProviderAdminUserName,omitempty"`
	// IdentityProviderPassword is a desired password of Keycloak admin user (applicable only when externalIdentityProvider is false)
	// +optional
	IdentityProviderPassword string `json:"identityProviderPassword,omitempty"`
	// IdentityProviderRealm is name of a keycloak realm. When externalIdentityProvider is false this realm will be created, otherwise passed to Che server
	// +optional
	IdentityProviderRealm string `json:"identityProviderRealm,omitempty"`
	// IdentityProviderClientId is id of a keycloak client. When externalIdentityProvider is false this client will be created, otherwise passed to Che server
	// +optional
	IdentityProviderClientId string `json:"identityProviderClientId,omitempty"`
	// IdentityProviderPostgresPassword is password for keycloak database user. Auto generated if left blank
	// +optional
	IdentityProviderPostgresPassword string `json:"identityProviderPostgresPassword,omitempty"`
	// UpdateAdminPassword forces the default admin Che user to update password on first login. False by default
	// +optional
	UpdateAdminPassword bool `json:"updateAdminPassword,omitempty"`
	// OpenShiftOauth instructs an Operator to enable OpenShift v3 identity provider in Keycloak,
	// as well as create respective oAuthClient and configure Che configMap accordingly
	// +optional
	OpenShiftoAuth bool `json:"openShiftoAuth,omitempty"`
	// OauthClientName is name of oAuthClient used in OpenShift v3 identity provider in Keycloak realm. Auto generated if left blank
	// +optional
	OAuthClientName string `json:"oAuthClientName,omitempty"`
	// OauthSecret is secret used in oAuthClient. Auto generated if left blank
	// +optional
	OAuthSecret string `json:"oAuthSecret,omitempty"`
	// IdentityProviderImage is image:tag used in Keycloak deployment
	// +optional
	IdentityProviderImage string `json:"identityProviderImage,omitempty"`
	// IdentityProviderImagePullPolicy is the image pull policy used in Keycloak registry deployment: default value is Always
	// +optional
	IdentityProviderImagePullPolicy corev1.PullPolicy `json:"identityProviderImagePullPolicy,omitempty"`
}

// +k8s:openapi-gen=true
type CheClusterSpecStorage struct {
	// PvcStrategy is a persistent volume claim strategy for Che server. Can be common (all workspaces PVCs in one volume),
	// per-workspace (one PVC per workspace for all declared volumes) and unique (one PVC per declared volume). Defaults to common
	// +optional
	PvcStrategy string `json:"pvcStrategy,omitempty"`
	// PvcClaimSize is size of a persistent volume claim for workspaces. Defaults to 1Gi
	// +optional
	PvcClaimSize string `json:"pvcClaimSize,omitempty"`
	// PreCreateSubPaths instructs Che server to launch a special pod to precreate a subpath in a PV
	// +optional
	PreCreateSubPaths bool `json:"preCreateSubPaths,omitempty"`
	// PvcJobsImage is image:tag for preCreateSubPaths jobs
	// +optional
	PvcJobsImage string `json:"pvcJobsImage,omitempty"`
	// PostgresPVCStorageClassName is storage class for a postgres pvc. Empty string by default, which means default storage class is used
	// +optional
	PostgresPVCStorageClassName string `json:"postgresPVCStorageClassName,omitempty"`
	// WorkspacePVCStorageClassName is storage class for a workspaces pvc. Empty string by default, which means default storage class is used
	// +optional
	WorkspacePVCStorageClassName string `json:"workspacePVCStorageClassName,omitempty"`
}

// +k8s:openapi-gen=true
type CheClusterSpecK8SOnly struct {
	// IngressDomain is a global ingress domain for a k8s cluster. Must be explicitly specified in CR. There are no defaults
	// +optional
	IngressDomain string `json:"ingressDomain,omitempty"`
	// IngressStrategy is the way ingresses are created. Casn be multi-host (host is explicitly provided in ingress),
	// single-host (host is provided, path based rules) and default-host *(no host is provided, path based rules)
	// +optional
	IngressStrategy string `json:"ingressStrategy,omitempty"`
	// IngressClass is kubernetes.io/ingress.class, defaults to nginx
	// +optional
	IngressClass string `json:"ingressClass,omitempty"`
	// secret name used for tls termination
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
	// FSGroup the Che POD and Workspace pod containers should run in
	// +optional
	SecurityContextFsGroup string `json:"securityContextFsGroup,omitempty"`
	// User the Che POD and Workspace pod containers should run as
	// +optional
	SecurityContextRunAsUser string `json:"securityContextRunAsUser,omitempty"`
}

// CheClusterStatus defines the observed state of CheCluster
type CheClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// DbProvisoned indicates if or not a Postgres instance has been provisioned with db and user
	DbProvisoned bool `json:"dbProvisioned"`
	// KeycloakProvisoned indicates if or not a Keycloak instance has been provisioned with realm, client, user
	KeycloakProvisoned bool `json:"keycloakProvisioned"`
	// OpenShiftoAuthProvisioned indicates if or not a Keycloak instance has been provisioned identity provider and oAuthclient
	OpenShiftoAuthProvisioned bool `json:"openShiftoAuthProvisioned"`
	// CheClusterRunning is status of a cluster. Can be Available, Unavailable, Available, Rolling Update in Progress
	CheClusterRunning string `json:"cheClusterRunning"`
	// CheVersion is current Che version retrieved from image tag
	CheVersion string `json:"cheVersion"`
	// CheURL is Che protocol+route/ingress
	CheURL string `json:"cheURL"`
	// KeycloakURL is Keycloak protocol+route/ingress
	KeycloakURL string `json:"keycloakURL"`
	// DevfileRegistryURL is the Devfile registry protocol+route/ingress
	DevfileRegistryURL string `json:"devfileRegistryURL"`
	// PluginRegistryURL is the Plugin registry protocol+route/ingress
	PluginRegistryURL string `json:"pluginRegistryURL"`
	// A human readable message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// e.g. 'Evicted'
	// +optional
	Reason string `json:"reason,omitempty"`
	// A URL that can point to some URL where to find help related to the current Operator status.
	// +optional
	HelpLink string `json:"helpLink,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheCluster is the Schema for the ches API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type CheCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheClusterSpec   `json:"spec,omitempty"`
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
