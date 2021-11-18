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

package v2alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// CheClusterSpec holds the configuration of the Che controller.
// +k8s:openapi-gen=true
type CheClusterSpec struct {
	// If false, Che is disabled and does not resolve the devworkspaces with the che routingClass.
	Enabled *bool `json:"enabled,omitempty"`

	// Configuration of the workspace endpoints that are exposed on separate domains, as opposed to the subpaths
	// of the gateway.
	WorkspaceDomainEndpoints WorkspaceDomainEndpoints `json:"workspaceDomainEndpoints,omitempty"`

	// Gateway contains the configuration of the gateway used for workspace endpoint routing.
	Gateway CheGatewaySpec `json:"gateway,omitempty"`

	// K8s contains the configuration specific only to Kubernetes
	K8s CheClusterSpecK8s `json:"k8s,omitempty"`
}

type WorkspaceDomainEndpoints struct {
	// The workspace endpoints that need to be deployed on a subdomain will be deployed on subdomains of this base domain.
	// This is mandatory on Kubernetes. On OpenShift, an attempt is made to automatically figure out the base domain of
	// the routes. The resolved value of this property is written to the status.
	BaseDomain string `json:"baseDomain,omitempty"`

	// Name of a secret that will be used to setup ingress/route TLS certificate for the workspace endpoints. The endpoints
	// will be on randomly named subdomains of the `BaseDomain` and therefore the TLS certificate should a wildcard certificate.
	//
	// When the field is empty string, the endpoints are served over unecrypted HTTP. This might be OK because the workspace endpoints
	// generally only expose applications in debugging sessions during development.
	//
	// The secret is assumed to exist in the same namespace as the CheCluster CR is copied to the namespace of the devworkspace on
	// devworkspace start (so that the ingresses on Kubernetes can reference it).
	//
	// The secret has to be of type "tls".
	//
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
}

type CheGatewaySpec struct {
	// Enabled enables or disables routing of the url rewrite supporting devworkspace endpoints
	// through a common gateway (the hostname of which is defined by the Host).
	//
	// Default value is "true" meaning that the gateway is enabled.
	//
	// If set to true (i.e. the gateway is enabled), endpoints marked using the "urlRewriteSupported" attribute
	// are exposed on unique subpaths of the Host, while the rest of the devworkspace endpoints are exposed
	// on subdomains of the Host.
	//
	// If set to false (i.e. the gateway is disabled), all endpoints are deployed on subdomains of
	// the Host.
	Enabled *bool `json:"enabled,omitempty"`

	// Host is the full host name used to expose devworkspace endpoints that support url rewriting reverse proxy.
	// See the gateway.enabled attribute for a more detailed description of where and how are devworkspace endpoints
	// exposed in various configurations.
	//
	// This attribute is mandatory on Kubernetes, optional on OpenShift.
	Host string `json:"host,omitempty"`

	// Name of a secret that will be used to setup ingress/route TLS certificate for the gateway host.
	// When the field is empty string, the default cluster certificate will be used.
	// The secret is assumed to exist in the same namespace as the CheCluster CR.
	//
	// The secret has to be of type "tls".
	//
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`

	// Image is the docker image to use for the Che gateway.  This is only used if Enabled is true.
	// If not defined in the CR, it is taken from
	// the `RELATED_IMAGE_gateway` environment variable of the operator deployment/pod. If not defined there,
	// it defaults to a hardcoded value.
	Image string `json:"image,omitempty"`

	// ConfigurerImage is the docker image to use for the sidecar of the Che gateway that is
	// used to configure it. This is only used when Enabled is true. If not defined in the CR,
	// it is taken from the `RELATED_IMAGE_gateway_configurer` environment variable of the operator
	// deployment/pod. If not defined there, it defaults to a hardcoded value.
	ConfigurerImage string `json:"configurerImage,omitempty"`

	// ConfigLabels are labels that are put on the gateway configuration configmaps so that they are picked up
	// by the gateway configurer. The default value are labels: app=che,component=che-gateway-config
	// +optional
	ConfigLabels labels.Set `json:"configLabels,omitempty"`
}

// CheClusterSpecK8s contains the configuration options specific to Kubernetes only.
type CheClusterSpecK8s struct {
	// IngressAnnotations are the annotations to be put on the generated ingresses. This can be used to
	// configure the ingress class and the ingress-controller-specific behavior for both the gateway
	// and the ingresses created to expose the Devworkspace component endpoints.
	// When not specified, this defaults to:
	//
	//     kubernetes.io/ingress.class:                       "nginx"
	//     nginx.ingress.kubernetes.io/proxy-read-timeout:    "3600",
	//     nginx.ingress.kubernetes.io/proxy-connect-timeout: "3600",
	//     nginx.ingress.kubernetes.io/ssl-redirect:          "true"
	//
	// +optional
	IngressAnnotations map[string]string `json:"ingressAnnotations,omitempty"`
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

// CheClusterStatusV2Alpha1 contains the status of the CheCluster object
// +k8s:openapi-gen=true
type CheClusterStatusV2Alpha1 struct {
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

// CheCluster is the configuration of the CheCluster layer of Devworkspace.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=checlusters,scope=Namespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +operator-sdk:csv:customresourcedefinitions:displayName="Eclipse Che Cluster"
type CheCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheClusterSpec           `json:"spec,omitempty"`
	Status CheClusterStatusV2Alpha1 `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CheClusterList is the list type for CheCluster
type CheClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheCluster{}, &CheClusterList{})
}

// IsEnabled is a utility method checking the `Enabled` property using its optional value or the default.
func (s *CheClusterSpec) IsEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

// IsEnabled is a utility method checking the `Enabled` property using its optional value or the default.
func (s *CheGatewaySpec) IsEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}
