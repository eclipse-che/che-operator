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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CheClusterSpec defines the desired state of CheCluster
type CheClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Server   CheClusterSpecServer  `json:"server"`
	Database CheClusterSpecDB      `json:"database"`
	Auth     CheClusterSpecAuth    `json:"auth"`
	Storage  CheClusterSpecStorage `json:"storage"`
	K8SOnly  CheClusterSpecK8SOnly `json:"k8s"`
}

type CheClusterSpecServer struct {
	// CheImage is a server image used in Che deployment
	CheImage string `json:"cheImage"`
	// CheImageTag is a tag of an image used in Che deployment
	CheImageTag string `json:"cheImageTag"`
	// CheFlavor is an installation flavor. Can be 'che' - upstream or 'codeready' - CodeReady Workspaces. Defaults to 'che'
	CheFlavor string `json:"cheFlavor"`
	// CheHost is an env consumer by server. Detected automatically from Che route
	CheHost string `json:"cheHost"`
	// CheLostLevel is log level for Che server: INFO or DEBUG. Defaults to INFO
	CheLogLevel string `json:"cheLogLevel"`
	// CheDebug is debug mode for Che server. Defaults to false
	CheDebug string `json:"cheDebug"`
	// CustomClusterRoleName specifies a custom cluster role to user for the Che workspaces
	// The default roles are used if this is left blank.
	CheWorkspaceClusterRole string `json:"cheWorkspaceClusterRole"`
	// SelfSignedCert signal about the necessity to get OpenShift router tls secret
	// and extract certificate to add it to Java trust store for Che server
	SelfSignedCert bool `json:"selfSignedCert"`
	// TlsSupport instructs an operator to deploy Che in TLS mode, ie with TLS routes or ingresses
	TlsSupport bool `json:"tlsSupport"`
	// PluginRegistryUrl is an endpoint serving plugin definitions. Defaults to https://che-plugin-registry.openshift.io
	PluginRegistryUrl string `json:"pluginRegistryUrl"`
	// ProxyURL is protocol+hostname of a proxy server. Automatically added as JAVA_OPTS and https(s)_proxy
	// to Che server and workspaces containers
	ProxyURL string `json:"proxyURL"`
	// ProxyPort is port of a proxy server
	ProxyPort string `json:"proxyPort"`
	// NonProxyHosts is a list of non-proxy hosts. Use | as delimiter, eg localhost|my.host.com|123.42.12.32
	NonProxyHosts string `json:"nonProxyHosts"`
	// ProxyUser is username for a proxy server
	ProxyUser string `json:"proxyUser"`
	// ProxyPassword is password for a proxy user
	ProxyPassword string `json:"proxyPassword"`
	// ServerMemoryRequest sets mem request for server deployment. Defaults to 512Mi
	ServerMemoryRequest string `json:"serverMemoryRequest"`
	// ServerMemoryLimit sets mem limit for server deployment. Defaults to 1Gi
	ServerMemoryLimit string `json:"serverMemoryLimit"`
}

type CheClusterSpecDB struct {
	// ExternalDB instructs the operator either to skip deploying Postgres,
	// and passes connection details of existing DB to Che server (when set to true)
	// or a new Postgres deployment is created
	ExternalDB bool `json:"externalDb"`
	// ChePostgresDBHostname is Postgres Database hostname that Che server uses to connect to. Defaults to postgres
	ChePostgresDBHostname string `json:"chePostgresHostName"`
	// ChePostgresPort is Postgres Database port that Che server uses to connect to. Defaults to 5432
	ChePostgresPort string `json:"chePostgresPort"`
	// ChePostgresUser is Postgres user that Che server when making a db connection. Defaults to pgche
	ChePostgresUser string `json:"chePostgresUser"`
	// ChePostgresPassword is password of a postgres user. Auto-generated when left blank
	ChePostgresPassword string `json:"chePostgresPassword"`
	// ChePostgresDb is Postgres database name that Che server uses to connect to. Defaults to dbche
	ChePostgresDb string `json:"chePostgresDb"`
	// PostgresImage is an image used in Postgres deployment in format image:tag. Defaults to registry.redhat.io/rhscl/postgresql-96-rhel7 (see pkg/deploy/defaults.go for latest tag)
	PostgresImage string `json:"postgresImage"`
}

type CheClusterSpecAuth struct {
	// ExternalKeycloak instructs operator on whether or not to deploy Keycloak/RH SSO instance. When set to true provision connection details
	ExternalKeycloak bool `json:"externalIdentityProvider"`
	// KeycloakURL is retrieved from respective route/ingress unless explicitly specified in CR (when externalIdentityProvider is true)
	KeycloakURL string `json:"identityProviderURL"`
	// KeycloakURL is retrieved from respective route/ingress unless explicitly specified in CR (when externalIdentityProvider is true)
	//IdentityProviderURL string `json:"identityProviderURL"`
	// KeycloakAdminUserName is a desired admin username of Keycloak admin user (applicable only when externalIdentityProvider is false)
	KeycloakAdminUserName string `json:"identityProviderAdminUserName"`
	// KeycloakAdminPassword is a desired password of Keycloak admin user (applicable only when externalIdentityProvider is false)
	KeycloakAdminPassword string `json:"identityProviderPassword"`
	// KeycloakRealm is name of a keycloak realm. When externalIdentityProvider is false this realm will be created, otherwise passed to Che server
	KeycloakRealm string `json:"identityProviderRealm"`
	// KeycloakClientId is id of a keycloak client. When externalIdentityProvider is false this client will be created, otherwise passed to Che server
	KeycloakClientId string `json:"identityProviderClientId"`
	// KeycloakPostgresPassword is password for keycloak database user. Auto generated if left blank
	KeycloakPostgresPassword string `json:"identityProviderPostgresPassword"`
	// UpdateAdminPassword forces the default admin Che user to update password on first login. False by default
	UpdateAdminPassword bool `json:"updateAdminPassword"`
	// OpenShiftOauth instructs an Operator to enable OpenShift v3 identity provider in Keycloak,
	// as well as create respective oAuthClient and configure Che configMap accordingly
	OpenShiftOauth bool `json:"openShiftoAuth"`
	// OauthClientName is name of oAuthClient used in OpenShift v3 identity provider in Keycloak realm. Auto generated if left blank
	OauthClientName string `json:"oAuthClientName"`
	// OauthSecret is secret used in oAuthClient. Auto generated if left blank
	OauthSecret string `json:"oAuthSecret"`
	// KeycloakImage is image:tag used in Keycloak deployment
	KeycloakImage string `json:"identityProviderImage"`
}

type CheClusterSpecStorage struct {
	// PvcStrategy is a persistent volume claim strategy for Che server. Can be common (all workspaces PVCs in one volume),
	// per-workspace (one PVC per workspace for all declared volumes) and unique (one PVC per declared volume). Defaults to common
	PvcStrategy string `json:"pvcStrategy"`
	// PvcClaimSize is size of a persistent volume claim for workspaces. Defaults to 1Gi
	PvcClaimSize string `json:"pvcClaimSize"`
	// PreCreateSubPaths instructs Che server to launch a special pod to precreate a subpath in a PV
	PreCreateSubPaths bool `json:"preCreateSubPaths"`
	// PvcJobsImage is image:tag for preCreateSubPaths jobs
	PvcJobsImage string `json:"pvcJobsImage"`
	// PostgresPVCStorageClassName is storage class for a postgres pvc. Empty string by default, which means default storage class is used
	PostgresPVCStorageClassName string `json:"postgresPVCStorageClassName"`
	// WorkspacePVCStorageClassName is storage class for a workspaces pvc. Empty string by default, which means default storage class is used
	WorkspacePVCStorageClassName string `json:"workspacePVCStorageClassName"`
}

type CheClusterSpecK8SOnly struct {
	// IngressDomain is a global ingress domain for a k8s cluster. Must be explicitly specified in CR. There are no defaults
	IngressDomain string `json:"ingressDomain"`
	// IngressStrategy is the way ingresses are created. Casn be multi-host (host is explicitly provided in ingress),
	// single-host (host is provided, path based rules) and default-host *(no host is provided, path based rules)
	IngressStrategy string `json:"ingressStrategy"`
	// IngressClass is kubernetes.io/ingress.class, defaults to nginx
	IngressClass string `json:"ingressClass"`
	// secret name used for tls termination
	TlsSecretName string `json:"tlsSecretName"`
	// FSGroup the Che POD and Workspace pod containers should run in  
	SecurityContextFsGroup string `json:"securityContextFsGroup"` 
	// User the Che POD and Workspace pod containers should run as  
	SecurityContextRunAsUser string `json:"securityContextRunAsUser"` 
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
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheCluster is the Schema for the ches API
// +k8s:openapi-gen=true
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
