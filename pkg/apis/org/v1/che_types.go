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
	Server CheClusterSpecServer `json:"server"`
	Database CheClusterSpecDB `json:"database"`
	Auth CheClusterSpecAuth `json:"auth"`
	Storage CheClusterSpecStorage `json:"storage"`
	K8SOnly CheClusterSpecK8SOnly `json:"k8s"`
}

type CheClusterSpecServer struct {
	CheImage string `json:"cheImage"`
	CheImageTag string `json:"cheImageTag"`
	CheFlavor string `json:"cheFlavor"`
	CheHost string `json:"cheHost"`
	CheLogLevel string `json:"cheLogLevel"`
	CheDebug string `json:"cheDebug"`
	SelfSignedCert bool `json:"selfSignedCert"`
	TlsSupport bool `json:"tlsSupport"`
	PluginRegistryUrl string `json:"pluginRegistryUrl"`
	ProxyURL string `json:"proxyURL"`
	ProxyPort string `json:"proxyPort"`
	NonProxyHosts string `json:"nonProxyHosts"`
	ProxyUser string `json:"proxyUser"`
	ProxyPassword string `json:"proxyPassword"`
}

type CheClusterSpecDB struct {

	ExternalDB bool `json:"externalDb"`
	ChePostgresDBHostname string `json:"chePostgresHostName"`
	ChePostgresPort string `json:"chePostgresPort"`
	ChePostgresUser string `json:"chePostgresUser"`
	ChePostgresPassword string `json:"chePostgresPassword"`
	ChePostgresDb string `json:"chePostgresDb"`
	PostgresImage string `json:"postgresImage"`
}

type CheClusterSpecAuth struct {

	ExternalKeycloak bool `json:"externalKeycloak"`
	KeycloakURL string `json:"keycloakURL"`
	KeycloakAdminUserName string `json:"keycloakAdminUserName"`
	KeycloakAdminPassword string `json:"keycloakAdminPassword"`
	KeycloakRealm string `json:"keycloakRealm"`
	KeycloakClientId string `json:"keycloakClientId"`
	KeycloakPostgresPassword string `json:"keycloakPostgresPassword"`
	UpdateAdminPassword bool `json:"updateAdminPassword"`
	OpenShiftOauth bool `json:"openShiftoAuth"`
	OauthClientName string `json:"oAuthClientName"`
	OauthSecret string `json:"oAuthSecret"`
	KeycloakImage string `json:"keycloakImage"`
}


type CheClusterSpecStorage struct {
	PvcStrategy string `json:"pvcStrategy"`
	PvcClaimSize string `json:"pvcClaimSize"`
	PreCreateSubPaths bool `json:"preCreateSubPaths"`
	PvcJobsImage string `json:"pvcJobsImage"`
}

type CheClusterSpecK8SOnly struct {
	IngressDomain string `json:"ingressDomain"`
	IngressStrategy string `json:"ingressStrategy"`
	IngressClass string `json:"ingressClass"`
	TlsSecretName string `json:"tlsSecretName"`
}

// CheClusterStatus defines the observed state of CheCluster
type CheClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	DbProvisoned bool `json:"dbProvisioned"`
	KeycloakProvisoned bool `json:"keycloakProvisioned"`
	OpenShiftoAuthProvisioned bool `json:"openShiftoAuthProvisioned"`
	CheClusterRunning string `json:"cheClusterRunning"`
	CheVersion string `json:"cheVersion"`
	CheURL string `json:"cheURL"`
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
