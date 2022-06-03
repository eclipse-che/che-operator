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

package v2

// Important: You must regenerate some generated code after modifying this file. At the root of the project:
// Run `make generate`. It will perform required changes:
// - update `api/v1/zz_generatedxxx` files;
// - update `config/crd/bases/org.eclipse.che_checlusters.yaml`

import (
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	imagepullerv1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:openapi-gen=true
// Desired configuration of Eclipse Che installation.
type CheClusterSpec struct {
	// Development environment default configuration options.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Development environments"
	DevEnvironments CheClusterDevEnvironments `json:"devEnvironments"`
	// Che components configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Components"
	Components CheClusterComponents `json:"components"`
	// Networking, Che authentication and TLS configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=3
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Networking"
	Networking CheClusterSpecNetworking `json:"networking,omitempty"`
	// Configuration of an alternative registry that stores Che images.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=4
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container registry"
	ContainerRegistry CheClusterContainerRegistry `json:"containerRegistry"`
}

// Development environments configuration.
// +k8s:openapi-gen=true
type CheClusterDevEnvironments struct {
	// Workspaces persistent storage.
	// +optional
	Storage WorkspaceStorage `json:"storage"`
	// Default plug-ins applied to Dev Workspaces.
	// +optional
	DefaultPlugins []WorkspaceDefaultPlugins `json:"defaultPlugins,omitempty"`
	// The node selector that limits the nodes that can run the workspace pods.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// The pod tolerations put on the workspace pods to limit where the workspace pods can run.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// User's default namespace.
	// +optional
	DefaultNamespace DefaultNamespace `json:"defaultNamespace,omitempty"`
	// Trusted certificate settings.
	// +optional
	TrustedCerts TrustedCerts `json:"trustedCerts,omitempty"`
}

// Che components configuration.
// +k8s:openapi-gen=true
type CheClusterComponents struct {
	// DevWorkspace operator configuration.
	// +optional
	DevWorkspace DevWorkspace `json:"devWorkspace"`
	// General configuration settings related to the Che server.
	// +optional
	CheServer CheServer `json:"cheServer"`
	// Configuration settings related to the Plugin registry used by the Che installation.
	// +optional
	PluginRegistry PluginRegistry `json:"pluginRegistry"`
	// Configuration settings related to the Devfile registry used by the Che installation.
	// +optional
	DevfileRegistry DevfileRegistry `json:"devfileRegistry"`
	// Configuration settings related to the database used by the Che installation.
	// +optional
	Database Database `json:"database"`
	// Configuration settings related to the Dashboard used by the Che installation.
	// +optional
	Dashboard Dashboard `json:"dashboard"`
	// Kubernetes Image Puller configuration.
	// +optional
	ImagePuller ImagePuller `json:"imagePuller"`
	// Che server metrics configuration.
	// +optional
	Metrics ServerMetrics `json:"metrics"`
}

// Configuration settings related to the Networking used by the Che installation.
// +k8s:openapi-gen=true
type CheClusterSpecNetworking struct {
	// List of labels that can be used to organize and categorize objects by scoping and selecting.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Unstructured key value map stored with a resource that may be set by external tools to store and retrieve arbitrary metadata.
	// When not specified, this defaults to:
	//     kubernetes.io/ingress.class:                       "nginx"
	//     nginx.ingress.kubernetes.io/proxy-read-timeout:    "3600",
	//     nginx.ingress.kubernetes.io/proxy-connect-timeout: "3600",
	//     nginx.ingress.kubernetes.io/ssl-redirect:          "true"
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// For OpenShift cluster Operator uses the domain to generate a hostname for a route.
	// The generated hostname will follow this pattern: eclipse-che.<domain>, where <che-namespace> is the namespace where the CheCluster CRD is created.
	// In a conjunction with labels it creates a route, which is served by a non-default Ingress controller.
	// For Kubernetes cluster it contains a global ingress domain. This MUST be explicitly specified: there are no defaults.
	// +optional
	Domain string `json:"domain,omitempty"`
	// Public hostname of the installed Che server.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Name of a secret that will be used to set up ingress TLS termination.
	// When the field is empty string, the default cluster certificate will be used.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
	// Authentication settings.
	// +optional
	Auth Auth `json:"auth"`
}

// Container registry configuration.
// +k8s:openapi-gen=true
type CheClusterContainerRegistry struct {
	// Optional hostname, or URL, to an alternate container registry to pull images from.
	// This value overrides the container registry hostname defined in all the default container images involved in a Che deployment.
	// This is particularly useful to install Che in a restricted environment.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// Optional repository name of an alternate container registry to pull images from.
	// This value overrides the container registry organization defined in all the default container images involved in a Che deployment.
	// This is particularly useful to install Eclipse Che in a restricted environment.
	// +optional
	Organization string `json:"organization,omitempty"`
}

// +k8s:openapi-gen=true
// General configuration settings related to the Che server.
type CheServer struct {
	// Deployment override options.
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// Log level for the Che server: `INFO` or `DEBUG`.
	// +optional
	// +kubebuilder:default:="INFO"
	LogLevel string `json:"logLevel,omitempty"`
	// Enables the debug mode for Che server.
	// +optional
	Debug *bool `json:"debug,omitempty"`
	// ClusterRoles that will be assigned to Che ServiceAccount.
	// Defaults roles are:
	// - `<che-namespace>-cheworkspaces-namespaces-clusterrole`
	// - `<che-namespace>-cheworkspaces-clusterrole`
	// - `<che-namespace>-cheworkspaces-devworkspace-clusterrole`
	// where <che-namespace> is the namespace where the CheCluster CRD is created.
	// Each role must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// Be aware that the Che Operator has to already have all permissions in these ClusterRoles to grant them.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty"`
	// Proxy server settings for Kubernetes cluster. No additional configuration is required for OpenShift cluster.
	// Specifying these settings for OpenShift cluster leads to overriding OpenShift proxy configuration.
	// +optional
	Proxy Proxy `json:"proxy"`
	// Map of additional environment variables that will be applied in the generated `che` ConfigMap to be used by the Che server,
	// in addition to the values already generated from other fields of the `CheCluster` custom resource (CR).
	// When `extraProperties` contains a property that would be normally generated in `che` ConfigMap from other CR fields,
	// the value defined in the `extraProperties` is used instead.
	// +optional
	ExtraProperties map[string]string `json:"extraProperties,omitempty"`
}

// Configuration settings related to the Dashaboard used by the Che installation.
// +k8s:openapi-gen=true
type Dashboard struct {
	// Deployment override options.
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// Dashboard header message.
	// +optional
	HeaderMessage DashboardHeaderMessage `json:"HeaderMessage,omitempty"`
}

// Configuration settings related to the Plugin Registry used by the Che installation.
// +k8s:openapi-gen=true
type PluginRegistry struct {
	// Deployment override options.
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// Disables internal Plugin registry.
	// +optional
	DisableInternalRegistry bool `json:"disableInternalRegistry,omitempty"`
	// External plugin registries.
	// Configure this in addition to a dedicated plugin registry (when `disableInternalRegistry` is `false`)
	// or instead of it (when `disableInternalRegistry` is `true`)
	// +optional
	ExternalPluginRegistries []ExternalPluginRegistry `json:"externalPluginRegistries,omitempty"`
}

// Configuration settings related to the Devfile Registry used by the Che installation.
// +k8s:openapi-gen=true
type DevfileRegistry struct {
	// Deployment override options.
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// Disables internal Devfile registry.
	// +optional
	DisableInternalRegistry bool `json:"disableInternalRegistry,omitempty"`
	// External devfile registries, that serves sample, ready-to-use devfiles.
	// Configure this in addition to a dedicated devfile registry (when `disableInternalRegistry` is `false`)
	// or instead of it (when `disableInternalRegistry` is `true`)
	// +optional
	ExternalDevfileRegistries []ExternalDevfileRegistry `json:"externalDevfileRegistries,omitempty"`
}

// Configuration settings related to the database used by the Che installation.
// +k8s:openapi-gen=true
type Database struct {
	// Instructs the Operator on whether to deploy a dedicated database.
	// By default, a dedicated PostgreSQL database is deployed as part of the Che installation. When `externalDb` is `true`, no dedicated database will be deployed by the
	// Operator and you will need to provide connection details to the external DB you are about to use.
	// +optional
	ExternalDb bool `json:"externalDb"`
	// Deployment override options.
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// PostgreSQL Database hostname that the Che server uses to connect to.
	// Override this value ONLY when using an external database. See field `externalDb`.
	// +kubebuilder:default:="postgres"
	// +optional
	PostgresHostName string `json:"postgresHostName,omitempty"`
	// PostgreSQL Database port that the Che server uses to connect to. Defaults to 5432.
	// Override this value ONLY when using an external database. See field `externalDb`. In the default case it will be automatically set by the Operator.
	// +optional
	// +kubebuilder:default:="5432"
	PostgresPort string `json:"postgresPort,omitempty"`
	// PostgreSQL database name that the Che server uses to connect to the DB.
	// +optional
	// +kubebuilder:default:="dbche"
	PostgresDb string `json:"postgresDb,omitempty"`
	// The secret that contains PostgreSQL `user` and `password` that the Che server uses to connect to the DB.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	// +kubebuilder:default:="postgres-credentials"
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
	// PVC settings for PostgreSQL database.
	// +optional
	Pvc PVC `json:"pvc,omitempty"`
}

// Che server metrics configuration
type ServerMetrics struct {
	// Enables `metrics` the Che server endpoint.
	// +kubebuilder:default:=true
	// +optional
	Enable bool `json:"enable"`
}

// Configuration settings for installation and configuration of the Kubernetes Image Puller
// See https://github.com/che-incubator/kubernetes-image-puller-operator
// +k8s:openapi-gen=true
type ImagePuller struct {
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
	Spec imagepullerv1alpha1.KubernetesImagePullerSpec `json:"spec"`
}

// Settings for installation and configuration of the DevWorkspace operator
// See https://github.com/devfile/devworkspace-operator
// +k8s:openapi-gen=true
type DevWorkspace struct {
	// Deployment override options.
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// Maximum number of the running workspaces per user.
	// +optional
	RunningLimit string `json:"runningLimit,omitempty"`
}

type DefaultNamespace struct {
	// if users namespaces are not pre-created this field defines the Kubernetes namespace created when a user starts their first workspace.
	// It's possible to use `<username>`, `<userid>` and `<workspaceid>` placeholders, such as che-workspace-<username>.
	// +optional
	Template string `json:"template,omitempty"`
}

type DashboardHeaderMessage struct {
	// Instructs dashboard to show the message.
	// +optional
	Show bool `json:"show,omitempty"`
	// Warning message that will be displayed on the User Dashboard
	// +optional
	Text string `json:"warning,text"`
}

type TrustedCerts struct {
	// The ConfigMap containing certificates to propagate to the Che components and to provide particular configuration for Git.
	// Note, this ConfigMap must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	GitTrustedCertsConfigMapName string `json:"gitTrustedCertsConfigMapName,omitempty"`
}

// Configuration settings related to the workspaces persistent storage.
type WorkspaceStorage struct {
	// PVC settings.
	// +optional
	Pvc PVC `json:"pvc,omitempty"`
	// Persistent volume claim strategy for the Che server.
	// The only `common` strategy is supported (all workspaces PVCs in one volume).
	// See for details https://github.com/eclipse/che/issues/21185.
	// +optional
	// +kubebuilder:default:="common"
	PvcStrategy string `json:"pvcStrategy,omitempty"`
}

type WorkspaceDefaultPlugins struct {
	// The editor id to specify default plug-ins for.
	Editor string `json:"editor,omitempty"`
	// Default plug-in uris for the specified editor.
	Plugins []string `json:"plugins,omitempty"`
}

// Authentication settings.
type Auth struct {
	// Public URL of the Identity Provider server.
	IdentityProviderURL string `json:"identityProviderURL,omitempty"`
	// Name of the OpenShift `OAuthClient` resource used to setup identity federation on the OpenShift side.
	OAuthClientName string `json:"oAuthClientName,omitempty"`
	// Name of the secret set in the OpenShift `OAuthClient` resource used to setup identity federation on the OpenShift side.
	OAuthSecret string `json:"oAuthSecret,omitempty"`
	// Gateway settings.
	// +optional
	Gateway Gateway `json:"gateway,omitempty"`
}

// Gateway settings.
type Gateway struct {
	// Deployment override options.
	// Since gateway deployment consist of several containers, they must be distungish in the configuration by their names:
	// - `gateway`
	// - `configbump`
	// - `oauth-proxy`
	// - `kube-rbac-proxy`
	// +optional
	Deployment Deployment `json:"deployment,omitempty"`
	// Gate configuration labels.
	// +optional
	ConfigLabels map[string]string `json:"configLabels,omitempty"`
}

// Proxy server configuration.
type Proxy struct {
	// URL (protocol+hostname) of the proxy server.
	// Only use when configuring a proxy is required. Operator respects OpenShift cluster wide proxy configuration
	// and no additional configuration is required, but defining `url` in a custom resource leads to overrides the cluster proxy configuration
	// with fields `url`, `port` and `credentialsSecretName` from the custom resource.
	// See the doc https://docs.openshift.com/container-platform/4.4/networking/enable-cluster-wide-proxy.html. See also the `proxyPort` and `nonProxyHosts` fields.
	// +optional
	Url string `json:"url,omitempty"`
	// Port of the proxy server.
	// +optional
	Port string `json:"port,omitempty"`
	// List of hosts that will be reached directly, bypassing the proxy.
	// Specify wild card domain use the following form `.<DOMAIN>`, for example:
	//    - localhost
	//    - my.host.com
	//    - 123.42.12.32
	// Only use when configuring a proxy is required. Operator respects OpenShift cluster wide proxy configuration and no additional configuration is required,
	// but defining `nonProxyHosts` in a custom resource leads to merging non proxy hosts lists from the cluster proxy configuration and ones defined in the custom resources.
	// See the doc https://docs.openshift.com/container-platform/4.4/networking/enable-cluster-wide-proxy.html. See also the `proxyURL` fields.
	NonProxyHosts []string `json:"nonProxyHosts,omitempty"`
	// The secret name that contains `user` and `password` for a proxy server.
	// The secret must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
}

// PersistentVolumeClaim custom settings.
type PVC struct {
	// Persistent Volume Claim size. To update the claim size, Storage class that provisions it must support resize.
	// +optional
	ClaimSize string `json:"claimSize,omitempty"`
	// Storage class for the Persistent Volume Claim. When omitted or left blank, a default storage class is used.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// External devfile registries configuration.
type ExternalDevfileRegistry struct {
	// Public URL of the devfile registry that serves sample ready-to-use devfiles.
	// +optional
	Url string `json:"url,omitempty"`
}

// External plugin registries configuration.
type ExternalPluginRegistry struct {
	// Public URL of the plugin registry.
	// +optional
	Url string `json:"url,omitempty"`
}

// Deployment custom settings.
type Deployment struct {
	// A single application container.
	// +optional
	Containers []Container `json:"container,omitempty"`
	// Security options the pod should run with.
	// +optional
	SecurityContext PodSecurityContext `json:"securityContext,omitempty"`
}

// Container custom settings.
type Container struct {
	// Container name.
	// +optional
	Name string `json:"name,omitempty"`
	// Container image. Omit it or leave it empty to use the default container image provided by the Operator.
	// +optional
	Image string `json:"image,omitempty"`
	// Image pull policy. Default value is `Always` for `nightly`, `next` or `latest` images, and `IfNotPresent` in other cases.
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Compute Resources required by this container.
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`
}

// Describes the compute resource requirements.
type ResourceRequirements struct {
	// Requests describes the minimum amount of compute resources required.
	// +optional
	Requests ResourceList `json:"request,omitempty"`
	// Limits describes the maximum amount of compute resources allowed.
	// +optional
	Limits ResourceList `json:"limits,omitempty"`
}

// List of resources.
type ResourceList struct {
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// +optional
	Memory resource.Quantity `json:"memory,omitempty"`
	// CPU, in cores. (500m = .5 cores)
	// +optional
	Cpu resource.Quantity `json:"cpu,omitempty"`
}

// PodSecurityContext holds pod-level security attributes and common container settings.
type PodSecurityContext struct {
	// The UID to run the entrypoint of the container process. Default value is `1724`.
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`
	// A special supplemental group that applies to all containers in a pod. Default value is `1724`.
	// +optional
	FsGroup *int64 `json:"fsGroup,omitempty"`
}

// GatewayPhase describes the different phases of the Che gateway lifecycle
type GatewayPhase string

const (
	GatewayPhaseInitializing = "Initializing"
	GatewayPhaseEstablished  = "Established"
	GatewayPhaseInactive     = "Inactive"
)

// CheClusterPhase describes the different phases of the Che cluster lifecycle
type CheClusterPhase string

const (
	ClusterPhaseActive          = "Active"
	ClusterPhaseInactive        = "Inactive"
	ClusterPhasePendingDeletion = "PendingDeletion"
	RollingUpdate               = "RollingUpdate"
)

// CheClusterStatus defines the observed state of Che installation
type CheClusterStatus struct {
	// Specifies the phase in which the gateway deployment currently is.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Gateway phase"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	GatewayPhase GatewayPhase `json:"gatewayPhase,omitempty"`
	// Current installed Che version.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="displayName: Eclipse Che version"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	CheVersion string `json:"cheVersion"`
	// Public URL to the Che server.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Eclipse Che URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	CheURL string `json:"cheURL"`
	// Specifies the phase in which the Che deployment currently is.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="ChePhase"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	ChePhase CheClusterPhase `json:"chePhase,omitempty"`
	// Public URL to the internal devfile registry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Devfile registry URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	DevfileRegistryURL string `json:"devfileRegistryURL"`
	// Public URL to the internal plugin registry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Plugin registry URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	PluginRegistryURL string `json:"pluginRegistryURL"`
	// A human readable message indicating details about why the Che deployment is in this phase.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Message"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the Che deployment is in this phase.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Reason"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	Reason string `json:"reason,omitempty"`
	// PostgreSQL version image is in use.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="PostgreSQL version"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	PostgresVersion string `json:"postgresVersion,omitempty"`
	// The resolved workspace base domain. This is either the copy of the explicitly defined property of the
	// same name in the spec or, if it is undefined in the spec and we're running on OpenShift, the automatically
	// resolved basedomain for routes.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Workspace base domain"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	WorkspaceBaseDomain string `json:"workspaceBaseDomain,omitempty"`
}

// The `CheCluster` custom resource allows defining and managing Eclipse Che server installation.
// Based on these settings, the  Operator automatically creates and maintains several ConfigMaps:
// `che`, `plugin-registry`, `devfile-registry` that will contain the appropriate environment variables
// of the various components of the installation. These generated ConfigMaps must NOT be updated manually.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +operator-sdk:csv:customresourcedefinitions:displayName="Eclipse Che instance Specification"
// +operator-sdk:csv:customresourcedefinitions:order=0
// +operator-sdk:csv:customresourcedefinitions:resources={{Ingress,v1},{Route,v1},{ConfigMap,v1},{Service,v1},{Secret,v1},{Deployment,apps/v1},{Role,v1},{RoleBinding,v1},{ClusterRole,v1},{ClusterRoleBinding,v1}}
// +kubebuilder:storageversion
type CheCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Desired configuration of Eclipse Che installation.
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
	return c.Spec.ContainerRegistry.Hostname != "" || c.Spec.ContainerRegistry.Organization != ""
}

func (c *CheCluster) IsImagePullerSpecEmpty() bool {
	return c.Spec.Components.ImagePuller.Spec == (imagepullerv1alpha1.KubernetesImagePullerSpec{})
}

func (c *CheCluster) IsImagePullerImagesEmpty() bool {
	return len(c.Spec.Components.ImagePuller.Spec.Images) == 0
}

func (c *CheCluster) GetCheHost() string {
	if c.Status.CheURL != "" {
		return strings.TrimPrefix(c.Status.CheURL, "https://")
	}

	return c.Spec.Networking.Hostname
}

func (c *CheCluster) GetDefaultNamespace() string {
	if c.Spec.Components.CheServer.ExtraProperties != nil {
		k8sDefaultNamespace := c.Spec.Components.CheServer.ExtraProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"]
		if k8sDefaultNamespace != "" {
			return k8sDefaultNamespace
		}
	}

	if c.Spec.DevEnvironments.DefaultNamespace.Template != "" {
		return c.Spec.DevEnvironments.DefaultNamespace.Template
	}

	return "<username>-" + os.Getenv("CHE_FLAVOR")
}
