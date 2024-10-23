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

package v2

// Important: Run `make generate` at the root directory of the project
// to regenerate `api/v2/zz_generatedxxx` code after modifying this file.

import (
	"strconv"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/constants"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"

	imagepullerv1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = ctrl.Log.WithName("checluster")

// +k8s:openapi-gen=true
// Desired configuration of Eclipse Che installation.
type CheClusterSpec struct {
	// Development environment default configuration options.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Development environments"
	// +kubebuilder:default:={storage: {pvcStrategy: per-user}, defaultNamespace: {template: <username>-che, autoProvision: true}, secondsOfInactivityBeforeIdling:1800, secondsOfRunBeforeIdling:-1, startTimeoutSeconds:300, maxNumberOfWorkspacesPerUser:-1}
	DevEnvironments CheClusterDevEnvironments `json:"devEnvironments"`
	// Che components configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Components"
	// +kubebuilder:default:={cheServer: {logLevel: INFO, debug: false}, metrics: {enable: true}}
	Components CheClusterComponents `json:"components"`
	// A configuration that allows users to work with remote Git repositories.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=3
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Git Services"
	GitServices CheClusterGitServices `json:"gitServices"`
	// Networking, Che authentication, and TLS configuration.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=4
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Networking"
	// +kubebuilder:default:={auth: {gateway: {configLabels: {app: che, component: che-gateway-config}}}}
	Networking CheClusterSpecNetworking `json:"networking"`
	// Configuration of an alternative registry that stores Che images.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=5
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Container registry"
	ContainerRegistry CheClusterContainerRegistry `json:"containerRegistry"`
}

// Development environment configuration.
// +k8s:openapi-gen=true
type CheClusterDevEnvironments struct {
	//
	// GatewayContainer configuration.
	// +optional
	GatewayContainer *Container `json:"gatewayContainer,omitempty"`
	// Project clone container configuration.
	// +optional
	ProjectCloneContainer *Container `json:"projectCloneContainer,omitempty"`
	// Workspaces persistent storage.
	// +optional
	// +kubebuilder:default:={pvcStrategy: per-user}
	Storage WorkspaceStorage `json:"storage,omitempty"`
	// PersistUserHome defines configuration options for persisting the
	// user home directory in workspaces.
	// +optional
	PersistUserHome *PersistentHomeConfig `json:"persistUserHome,omitempty"`
	// Default plug-ins applied to DevWorkspaces.
	// +optional
	DefaultPlugins []WorkspaceDefaultPlugins `json:"defaultPlugins,omitempty"`
	// The node selector limits the nodes that can run the workspace pods.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// The pod tolerations of the workspace pods limit where the workspace pods can run.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// User's default namespace.
	// +optional
	// +kubebuilder:default:={template: <username>-che, autoProvision: true}
	DefaultNamespace DefaultNamespace `json:"defaultNamespace,omitempty"`
	// Trusted certificate settings.
	// +optional
	TrustedCerts *TrustedCerts `json:"trustedCerts,omitempty"`
	// The default editor to workspace create with. It could be a plugin ID or a URI.
	// The plugin ID must have `publisher/name/version` format.
	// The URI must start from `http://` or `https://`.
	// +optional
	DefaultEditor string `json:"defaultEditor,omitempty"`
	// Default components applied to DevWorkspaces.
	// These default components are meant to be used when a Devfile, that does not contain any components.
	// +optional
	DefaultComponents []devfile.Component `json:"defaultComponents,omitempty"`
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
	// Disables the container build capabilities.
	// When set to `false` (the default value), the devEnvironments.security.containerSecurityContext
	// field is ignored, and the following container SecurityContext is applied:
	//
	//  containerSecurityContext:
	//    allowPrivilegeEscalation: true
	//    capabilities:
	//      add:
	//      - SETGID
	//      - SETUID
	//
	// +optional
	DisableContainerBuildCapabilities *bool `json:"disableContainerBuildCapabilities,omitempty"`
	// Workspace security configuration.
	// +optional
	Security WorkspaceSecurityConfig `json:"security,omitempty"`
	// Container build configuration.
	// +optional
	ContainerBuildConfiguration *ContainerBuildConfiguration `json:"containerBuildConfiguration,omitempty"`
	// ServiceAccount to use by the DevWorkspace operator when starting the workspaces.
	// +optional
	// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
	// +kubebuilder:validation:MaxLength=63
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// List of ServiceAccount tokens that will be mounted into workspace pods as projected volumes.
	// +optional
	ServiceAccountTokens []controllerv1alpha1.ServiceAccountToken `json:"serviceAccountTokens,omitempty"`
	// Pod scheduler for the workspace pods.
	// If not specified, the pod scheduler is set to the default scheduler on the cluster.
	// +optional
	PodSchedulerName string `json:"podSchedulerName,omitempty"`
	// RuntimeClassName specifies the spec.runtimeClassName for workspace pods.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
	// StartTimeoutSeconds determines the maximum duration (in seconds) that a workspace can take to start
	// before it is automatically failed.
	// If not specified, the default value of 300 seconds (5 minutes) is used.
	// +optional
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:default:=300
	StartTimeoutSeconds *int32 `json:"startTimeoutSeconds,omitempty"`
	// DeploymentStrategy defines the deployment strategy to use to replace existing workspace pods
	// with new ones. The available deployment stragies are `Recreate` and `RollingUpdate`.
	// With the `Recreate` deployment strategy, the existing workspace pod is killed before the new one is created.
	// With the `RollingUpdate` deployment strategy, a new workspace pod is created and the existing workspace pod is deleted
	// only when the new workspace pod is in a ready state.
	// If not specified, the default `Recreate` deployment strategy is used.
	// +optional
	// +kubebuilder:validation:Enum=Recreate;RollingUpdate
	DeploymentStrategy appsv1.DeploymentStrategyType `json:"deploymentStrategy,omitempty"`
	// Total number of workspaces, both stopped and running, that a user can keep.
	// The value, -1, allows users to keep an unlimited number of workspaces.
	// +kubebuilder:validation:Minimum:=-1
	// +kubebuilder:default:=-1
	// +optional
	MaxNumberOfWorkspacesPerUser *int64 `json:"maxNumberOfWorkspacesPerUser,omitempty"`
	// The maximum number of running workspaces per user.
	// The value, -1, allows users to run an unlimited number of workspaces.
	// +kubebuilder:validation:Minimum:=-1
	// +optional
	MaxNumberOfRunningWorkspacesPerUser *int64 `json:"maxNumberOfRunningWorkspacesPerUser,omitempty"`
	// The maximum number of concurrently running workspaces across the entire Kubernetes cluster.
	// This applies to all users in the system. If the value is set to -1, it means there is
	// no limit on the number of running workspaces.
	// +kubebuilder:validation:Minimum:=-1
	// +optional
	MaxNumberOfRunningWorkspacesPerCluster *int64 `json:"maxNumberOfRunningWorkspacesPerCluster,omitempty"`
	// User configuration.
	// +optional
	User *UserConfiguration `json:"user,omitempty"`
	// ImagePullPolicy defines the imagePullPolicy used for containers in a DevWorkspace.
	// +optional
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// WorkspacesPodAnnotations defines additional annotations for workspace pods.
	// +optional
	WorkspacesPodAnnotations map[string]string `json:"workspacesPodAnnotations,omitempty"`
	// IgnoredUnrecoverableEvents defines a list of Kubernetes event names that should
	// be ignored when deciding to fail a workspace that is starting. This option should be used
	// if a transient cluster issue is triggering false-positives (for example, if
	// the cluster occasionally encounters FailedScheduling events). Events listed
	// here will not trigger workspace failures.
	// +kubebuilder:default:={"FailedScheduling"}
	// +optional
	IgnoredUnrecoverableEvents []string `json:"ignoredUnrecoverableEvents"`
	// AllowedSources defines the allowed sources on which workspaces can be started.
	// +optional
	AllowedSources *AllowedSources `json:"allowedSources,omitempty"`
}

// Che components configuration.
// +k8s:openapi-gen=true
type CheClusterComponents struct {
	// DevWorkspace Operator configuration.
	// +optional
	DevWorkspace DevWorkspace `json:"devWorkspace"`
	// General configuration settings related to the Che server.
	// +optional
	// +kubebuilder:default:={logLevel: INFO, debug: false}
	CheServer CheServer `json:"cheServer"`
	// Configuration settings related to the plug-in registry used by the Che installation.
	// +optional
	PluginRegistry PluginRegistry `json:"pluginRegistry"`
	// Configuration settings related to the devfile registry used by the Che installation.
	// +optional
	DevfileRegistry DevfileRegistry `json:"devfileRegistry"`
	// Configuration settings related to the dashboard used by the Che installation.
	// +optional
	Dashboard Dashboard `json:"dashboard"`
	// Kubernetes Image Puller configuration.
	// +optional
	ImagePuller ImagePuller `json:"imagePuller"`
	// Che server metrics configuration.
	// +optional
	// +kubebuilder:default:={enable: true}
	Metrics ServerMetrics `json:"metrics"`
}

// Configuration settings related to the networking used by the Che installation.
// +k8s:openapi-gen=true
type CheClusterSpecNetworking struct {
	// Defines labels which will be set for an Ingress (a route for OpenShift platform).
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// Defines annotations which will be set for an Ingress (a route for OpenShift platform).
	// The defaults for kubernetes platforms are:
	//     kubernetes.io/ingress.class:                       "nginx"
	//     nginx.ingress.kubernetes.io/proxy-read-timeout:    "3600",
	//     nginx.ingress.kubernetes.io/proxy-connect-timeout: "3600",
	//     nginx.ingress.kubernetes.io/ssl-redirect:          "true"
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// For an OpenShift cluster, the Operator uses the domain to generate a hostname for the route.
	// The generated hostname follows this pattern: che-<che-namespace>.<domain>. The <che-namespace> is the namespace where the CheCluster CRD is created.
	// In conjunction with labels, it creates a route served by a non-default Ingress controller.
	// For a Kubernetes cluster, it contains a global ingress domain. There are no default values: you must specify them.
	// +optional
	Domain string `json:"domain,omitempty"`
	// The public hostname of the installed Che server.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// The name of the secret used to set up Ingress TLS termination.
	// If the field is an empty string, the default cluster certificate is used.
	// The secret must have a `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	TlsSecretName string `json:"tlsSecretName,omitempty"`
	// IngressClassName is the name of an IngressClass cluster resource.
	// If a class name is defined in both the `IngressClassName` field and the `kubernetes.io/ingress.class` annotation,
	// `IngressClassName` field takes precedence.
	IngressClassName string `json:"ingressClassName,omitempty"`
	// Authentication settings.
	// +optional
	// +kubebuilder:default:={gateway: {configLabels: {app: che, component: che-gateway-config}}}
	Auth Auth `json:"auth"`
}

// Container registry configuration.
// +k8s:openapi-gen=true
type CheClusterContainerRegistry struct {
	// An optional hostname or URL of an alternative container registry to pull images from.
	// This value overrides the container registry hostname defined in all the default container images involved in a Che deployment.
	// This is particularly useful for installing Che in a restricted environment.
	// +optional
	Hostname string `json:"hostname,omitempty"`
	// An optional repository name of an alternative registry to pull images from.
	// This value overrides the container registry organization defined in all the default container images involved in a Che deployment.
	// This is particularly useful for installing Eclipse Che in a restricted environment.
	// +optional
	Organization string `json:"organization,omitempty"`
}

// +k8s:openapi-gen=true
// General configuration settings related to the Che server.
type CheServer struct {
	// Deployment override options.
	// +optional
	Deployment *Deployment `json:"deployment,omitempty"`
	// The log level for the Che server: `INFO` or `DEBUG`.
	// +optional
	// +kubebuilder:default:="INFO"
	LogLevel string `json:"logLevel,omitempty"`
	// Enables the debug mode for Che server.
	// +optional
	// +kubebuilder:default:=false
	Debug *bool `json:"debug,omitempty"`
	// Additional ClusterRoles assigned to Che ServiceAccount.
	// Each role must have a `app.kubernetes.io/part-of=che.eclipse.org` label.
	// The defaults roles are:
	// - `<che-namespace>-cheworkspaces-clusterrole`
	// - `<che-namespace>-cheworkspaces-namespaces-clusterrole`
	// - `<che-namespace>-cheworkspaces-devworkspace-clusterrole`
	// where the <che-namespace> is the namespace where the CheCluster CR is created.
	// The Che Operator must already have all permissions in these ClusterRoles to grant them.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty"`
	// Proxy server settings for Kubernetes cluster. No additional configuration is required for OpenShift cluster.
	// By specifying these settings for the OpenShift cluster, you override the OpenShift proxy configuration.
	// +optional
	Proxy *Proxy `json:"proxy,omitempty"`
	// A map of additional environment variables applied in the generated `che` ConfigMap to be used by the Che server
	// in addition to the values already generated from other fields of the `CheCluster` custom resource (CR).
	// If the `extraProperties` field contains a property normally generated in `che` ConfigMap from other CR fields,
	// the value defined in the `extraProperties` is used instead.
	// +optional
	ExtraProperties map[string]string `json:"extraProperties,omitempty"`
}

// Configuration settings related to the Dashaboard used by the Che installation.
// +k8s:openapi-gen=true
type Dashboard struct {
	// The log level for the Dashboard.
	// +optional
	// +kubebuilder:default:="ERROR"
	// +kubebuilder:validation:Enum=DEBUG;INFO;WARN;ERROR;FATAL;TRACE;SILENT
	LogLevel string `json:"logLevel,omitempty"`
	// Deployment override options.
	// +optional
	Deployment *Deployment `json:"deployment,omitempty"`
	// Dashboard header message.
	// +optional
	HeaderMessage *DashboardHeaderMessage `json:"headerMessage,omitempty"`
	// Dashboard branding resources.
	// +optional
	Branding *Branding `json:"branding,omitempty"`
}

type Branding struct {
	// Dashboard logo.
	// +optional
	Logo *Icon `json:"logo,omitempty"`
}

type Icon struct {
	Data      string `json:"base64data"`
	MediaType string `json:"mediatype"`
}

// Configuration settings related to the plug-in registry used by the Che installation.
// +k8s:openapi-gen=true
type PluginRegistry struct {
	// Deployment override options.
	// +optional
	Deployment *Deployment `json:"deployment,omitempty"`
	// Disables internal plug-in registry.
	// +optional
	DisableInternalRegistry bool `json:"disableInternalRegistry,omitempty"`
	// External plugin registries.
	// +optional
	ExternalPluginRegistries []ExternalPluginRegistry `json:"externalPluginRegistries,omitempty"`
	// Open VSX registry URL. If omitted an embedded instance will be used.
	// +optional
	OpenVSXURL *string `json:"openVSXURL,omitempty"`
}

// Configuration settings related to the devfile registry used by the Che installation.
// +k8s:openapi-gen=true
type DevfileRegistry struct {
	// Deprecated deployment override options.
	// +optional
	Deployment *Deployment `json:"deployment,omitempty"`
	// Disables internal devfile registry.
	// +optional
	DisableInternalRegistry bool `json:"disableInternalRegistry,omitempty"`
	// External devfile registries serving sample ready-to-use devfiles.
	// +optional
	ExternalDevfileRegistries []ExternalDevfileRegistry `json:"externalDevfileRegistries,omitempty"`
}

// Che server metrics configuration
type ServerMetrics struct {
	// Enables `metrics` for the Che server endpoint.
	// +optional
	// +kubebuilder:default:=true
	Enable bool `json:"enable"`
}

// Configuration settings for installation and configuration of the Kubernetes Image Puller
// See https://github.com/che-incubator/kubernetes-image-puller-operator
// +k8s:openapi-gen=true
type ImagePuller struct {
	// Install and configure the community supported Kubernetes Image Puller Operator. When you set the value to `true` without providing any specs,
	// it creates a default Kubernetes Image Puller object managed by the Operator.
	// When you set the value to `false`, the Kubernetes Image Puller object is deleted, and the Operator uninstalled,
	// regardless of whether a spec is provided.
	// If you leave the `spec.images` field empty, a set of recommended workspace-related images is automatically detected and
	// pre-pulled after installation.
	// Note that while this Operator and its behavior is community-supported, its payload may be commercially-supported
	// for pulling commercially-supported images.
	// +optional
	Enable bool `json:"enable"`
	// A Kubernetes Image Puller spec to configure the image puller in the CheCluster.
	// +optional
	Spec imagepullerv1alpha1.KubernetesImagePullerSpec `json:"spec"`
}

// Settings for installation and configuration of the DevWorkspace Operator
// See https://github.com/devfile/devworkspace-operator
// +k8s:openapi-gen=true
type DevWorkspace struct {
	// Deprecated in favor of `MaxNumberOfRunningWorkspacesPerUser`
	// The maximum number of running workspaces per user.
	// +optional
	RunningLimit string `json:"runningLimit,omitempty"`
}

type DefaultNamespace struct {
	// If you don't create the user namespaces in advance, this field defines the Kubernetes namespace created when you start your first workspace.
	// You can use `<username>` and `<userid>` placeholders, such as che-workspace-<username>.
	// +optional
	// +kubebuilder:default:=<username>-che
	// +kubebuilder:validation:Pattern=<username>|<userid>
	Template string `json:"template,omitempty"`
	// Indicates if is allowed to automatically create a user namespace.
	// If it set to false, then user namespace must be pre-created by a cluster administrator.
	// +optional
	// +kubebuilder:default:=true
	AutoProvision *bool `json:"autoProvision,omitempty"`
}

type DashboardHeaderMessage struct {
	// Instructs dashboard to show the message.
	// +optional
	Show bool `json:"show,omitempty"`
	// Warning message displayed on the user dashboard.
	// +optional
	Text string `json:"text,omitempty"`
}

type TrustedCerts struct {
	// The ConfigMap contains certificates to propagate to the Che components and to provide a particular configuration for Git.
	// See the following page: https://www.eclipse.org/che/docs/stable/administration-guide/deploying-che-with-support-for-git-repositories-with-self-signed-certificates/
	// The ConfigMap must have a `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	GitTrustedCertsConfigMapName string `json:"gitTrustedCertsConfigMapName,omitempty"`
}

type UserConfiguration struct {
	// Additional ClusterRoles assigned to the user.
	// The role must have `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	ClusterRoles []string `json:"clusterRoles,omitempty"`
}

// Configuration settings related to the workspaces persistent storage.
type WorkspaceStorage struct {
	// PVC settings when using the `per-user` PVC strategy.
	// +optional
	PerUserStrategyPvcConfig *PVC `json:"perUserStrategyPvcConfig,omitempty"`
	// PVC settings when using the `per-workspace` PVC strategy.
	// +optional
	PerWorkspaceStrategyPvcConfig *PVC `json:"perWorkspaceStrategyPvcConfig,omitempty"`
	// Persistent volume claim strategy for the Che server.
	// The supported strategies are: `per-user` (all workspaces PVCs in one volume),
	// `per-workspace` (each workspace is given its own individual PVC)
	// and `ephemeral` (non-persistent storage where local changes will be lost when
	// the workspace is stopped.)
	// +optional
	// +kubebuilder:default:="per-user"
	// +kubebuilder:validation:Enum=common;per-user;per-workspace;ephemeral
	PvcStrategy string `json:"pvcStrategy,omitempty"`
}

type PersistentHomeConfig struct {
	// Determines whether the user home directory in workspaces should persist between
	// workspace shutdown and startup.
	// Must be used with the 'per-user' or 'per-workspace' PVC strategy in order to take effect.
	// Disabled by default.
	Enabled *bool `json:"enabled,omitempty"`
	// Determines whether the init container that initializes the persistent home directory should be disabled.
	// When the `/home/user` directory is persisted, the init container is used to initialize the directory before
	// the workspace starts. If set to true, the init container will not be created.
	// Disabling the init container allows home persistence to be initialized by the entrypoint present in the workspace's first container component.
	// This field is not used if the `devEnvironments.persistUserHome.enabled` field is set to false.
	// The init container is enabled by default.
	DisableInitContainer *bool `json:"disableInitContainer,omitempty"`
}

type WorkspaceDefaultPlugins struct {
	// The editor ID to specify default plug-ins for.
	// The plugin ID must have `publisher/name/version` format.
	Editor string `json:"editor,omitempty"`
	// Default plug-in URIs for the specified editor.
	Plugins []string `json:"plugins,omitempty"`
}

// Workspace security configuration
type WorkspaceSecurityConfig struct {
	// PodSecurityContext used by all workspace-related pods.
	// If set, defined values are merged into the default PodSecurityContext configuration.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
	// Container SecurityContext used by all workspace-related containers.
	// If set, defined values are merged into the default Container SecurityContext configuration.
	// Requires devEnvironments.disableContainerBuildCapabilities to be set to `true` in order to take effect.
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`
}

// Authentication settings.
type Auth struct {
	// Public URL of the Identity Provider server.
	// +optional
	IdentityProviderURL string `json:"identityProviderURL,omitempty"`
	// Name of the OpenShift `OAuthClient` resource used to set up identity federation on the OpenShift side.
	// +optional
	OAuthClientName string `json:"oAuthClientName,omitempty"`
	// Name of the secret set in the OpenShift `OAuthClient` resource used to set up identity federation on the OpenShift side.
	// For Kubernetes, this can either be the plain text oAuthSecret value, or the name of a kubernetes secret which contains a
	// key `oAuthSecret` and the value is the secret. NOTE: this secret must exist in the same namespace as the `CheCluster`
	// resource and contain the label `app.kubernetes.io/part-of=che.eclipse.org`.
	// +optional
	OAuthSecret string `json:"oAuthSecret,omitempty"`
	// Access Token Scope.
	// This field is specific to Che installations made for Kubernetes only and ignored for OpenShift.
	// +optional
	OAuthScope string `json:"oAuthScope,omitempty"`
	// Inactivity timeout for tokens to set in the OpenShift `OAuthClient` resource used to set up identity federation on the OpenShift side.
	// 0 means tokens for this client never time out.
	// +optional
	OAuthAccessTokenInactivityTimeoutSeconds *int32 `json:"oAuthAccessTokenInactivityTimeoutSeconds,omitempty"`
	// Access token max age for tokens to set in the OpenShift `OAuthClient` resource used to set up identity federation on the OpenShift side.
	// 0 means no expiration.
	// +optional
	OAuthAccessTokenMaxAgeSeconds *int32 `json:"oAuthAccessTokenMaxAgeSeconds,omitempty"`
	// Identity token to be passed to upstream. There are two types of tokens supported: `id_token` and `access_token`.
	// Default value is `id_token`.
	// This field is specific to Che installations made for Kubernetes only and ignored for OpenShift.
	// +optional
	// +kubebuilder:validation:Enum=id_token;access_token
	IdentityToken string `json:"identityToken,omitempty"`
	// Gateway settings.
	// +optional
	// +kubebuilder:default:={configLabels: {app: che, component: che-gateway-config}}
	Gateway Gateway `json:"gateway,omitempty"`
	// Advance authorization settings. Determines which users and groups are allowed to access Che.
	// User is allowed to access Che if he/she is either in the `allowUsers` list or is member of group from `allowGroups` list
	// and not in neither the `denyUsers` list nor is member of group from `denyGroups` list.
	// If `allowUsers` and `allowGroups` are empty, then all users are allowed to access Che.
	// if `denyUsers` and `denyGroups` are empty, then no users are denied to access Che.
	// +optional
	AdvancedAuthorization *AdvancedAuthorization `json:"advancedAuthorization,omitempty"`
}

type AdvancedAuthorization struct {
	// List of users allowed to access Che.
	// +optional
	AllowUsers []string `json:"allowUsers,omitempty"`
	// List of groups allowed to access Che (currently supported in OpenShift only).
	// +optional
	AllowGroups []string `json:"allowGroups,omitempty"`
	// List of users denied to access Che.
	// +optional
	DenyUsers []string `json:"denyUsers,omitempty"`
	// List of groups denied to access Che (currently supported in OpenShift only).
	// +optional
	DenyGroups []string `json:"denyGroups,omitempty"`
}

// Gateway settings.
type Gateway struct {
	// Deployment override options.
	// Since gateway deployment consists of several containers, they must be distinguished in the configuration by their names:
	// - `gateway`
	// - `configbump`
	// - `oauth-proxy`
	// - `kube-rbac-proxy`
	// +optional
	Deployment *Deployment `json:"deployment,omitempty"`
	// Gateway configuration labels.
	// +optional
	// +kubebuilder:default:={app: che, component: che-gateway-config}
	ConfigLabels map[string]string `json:"configLabels,omitempty"`
	// Configuration for Traefik within the Che gateway pod.
	// +optional
	Traefik *Traefik `json:"traefik,omitempty"`
	// Configuration for kube-rbac-proxy within the Che gateway pod.
	// +optional
	KubeRbacProxy *KubeRbacProxy `json:"kubeRbacProxy,omitempty"`
	// Configuration for oauth-proxy within the Che gateway pod.
	// +optional
	OAuthProxy *OAuthProxy `json:"oAuthProxy,omitempty"`
}

type OAuthProxy struct {
	// Expire timeframe for cookie. If set to 0, cookie becomes a session-cookie which will expire when the browser is closed.
	// +optional
	// +kubebuilder:default:=86400
	// +kubebuilder:validation:Minimum:=0
	CookieExpireSeconds *int32 `json:"cookieExpireSeconds,omitempty"`
}

// Proxy server configuration.
type Proxy struct {
	// URL (protocol+hostname) of the proxy server.
	// Use only when a proxy configuration is required. The Operator respects OpenShift cluster-wide proxy configuration,
	// defining `url` in a custom resource leads to overriding the cluster proxy configuration.
	// See the following page: https://docs.openshift.com/container-platform/latest/networking/enable-cluster-wide-proxy.html.
	// +optional
	Url string `json:"url,omitempty"`
	// Proxy server port.
	// +optional
	Port string `json:"port,omitempty"`
	// A list of hosts that can be reached directly, bypassing the proxy.
	// Specify wild card domain use the following form `.<DOMAIN>`, for example:
	//    - localhost
	//    - 127.0.0.1
	//    - my.host.com
	//    - 123.42.12.32
	// Use only when a proxy configuration is required. The Operator respects OpenShift cluster-wide proxy configuration,
	// defining `nonProxyHosts` in a custom resource leads to merging non-proxy hosts lists from the cluster proxy configuration, and the ones defined in the custom resources.
	// See the following page: https://docs.openshift.com/container-platform/latest/networking/enable-cluster-wide-proxy.html.
	// In some proxy configurations, localhost may not translate to 127.0.0.1. Both localhost and 127.0.0.1 should be specified in this situation.
	// +optional
	NonProxyHosts []string `json:"nonProxyHosts,omitempty"`
	// The secret name that contains `user` and `password` for a proxy server.
	// The secret must have a `app.kubernetes.io/part-of=che.eclipse.org` label.
	// +optional
	CredentialsSecretName string `json:"credentialsSecretName,omitempty"`
}

// PersistentVolumeClaim custom settings.
type PVC struct {
	// Persistent Volume Claim size. To update the claim size, the storage class that provisions it must support resizing.
	// +optional
	ClaimSize string `json:"claimSize,omitempty"`
	// Storage class for the Persistent Volume Claim. When omitted or left blank, a default storage class is used.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// External devfile registries configuration.
type ExternalDevfileRegistry struct {
	// The public UR of the devfile registry that serves sample ready-to-use devfiles.
	// +optional
	Url string `json:"url,omitempty"`
}

// External plug-in registries configuration.
type ExternalPluginRegistry struct {
	// Public URL of the plug-in registry.
	// +optional
	Url string `json:"url,omitempty"`
}

// Deployment custom settings.
type Deployment struct {
	// List of containers belonging to the pod.
	// +optional
	Containers []Container `json:"containers,omitempty"`
	// Security options the pod should run with.
	// +optional
	SecurityContext *PodSecurityContext `json:"securityContext,omitempty"`
	// The node selector limits the nodes that can run the pod.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// The pod tolerations of the component pod limit where the pod can run.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
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
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Compute resources required by this container.
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// List of environment variables to set in the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// Describes the compute resource requirements.
type ResourceRequirements struct {
	// Describes the minimum amount of compute resources required.
	// +optional
	Requests *ResourceList `json:"request,omitempty"`
	// Describes the maximum amount of compute resources allowed.
	// +optional
	Limits *ResourceList `json:"limits,omitempty"`
}

// List of resources.
type ResourceList struct {
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// If the value is not specified, then the default value is set depending on the component.
	// If value is `0`, then no value is set for the component.
	// +optional
	Memory *resource.Quantity `json:"memory,omitempty"`
	// CPU, in cores. (500m = .5 cores)
	// If the value is not specified, then the default value is set depending on the component.
	// If value is `0`, then no value is set for the component.
	// +optional
	Cpu *resource.Quantity `json:"cpu,omitempty"`
}

// PodSecurityContext holds pod-level security attributes and common container settings.
type PodSecurityContext struct {
	// The UID to run the entrypoint of the container process. The default value is `1724`.
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`
	// A special supplemental group that applies to all containers in a pod. The default value is `1724`.
	// +optional
	FsGroup *int64 `json:"fsGroup,omitempty"`
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
	// Enables users to work with repositories hosted on Azure DevOps Service (dev.azure.com).
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Azure"
	AzureDevOps []AzureDevOpsService `json:"azure,omitempty"`
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
	// Disables subdomain isolation.
	// Deprecated in favor of `che.eclipse.org/scm-github-disable-subdomain-isolation` annotation.
	// See the following page for details: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-github/.
	// +optional
	DisableSubdomainIsolation *bool `json:"disableSubdomainIsolation,omitempty"`
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

// AzureDevOpsService enables users to work with repositories hosted on Azure DevOps Service (dev.azure.com).
type AzureDevOpsService struct {
	// Kubernetes secret, that contains Base64-encoded Azure DevOps Service Application ID and Client Secret.
	// See the following page: https://www.eclipse.org/che/docs/stable/administration-guide/configuring-oauth-2-for-microsoft-azure-devops-services
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors="urn:alm:descriptor:io.kubernetes:Secret"
	SecretName string `json:"secretName"`
}

// Container build configuration.
type ContainerBuildConfiguration struct {
	// OpenShift security context constraint to build containers.
	// +kubebuilder:validation:Required
	// +kubebuilder:default:=container-build
	OpenShiftSecurityContextConstraint string `json:"openShiftSecurityContextConstraint,omitempty"`
}

// Configuration for Traefik within the Che gateway pod.
type Traefik struct {
	// The log level for the Traefik container within the gateway pod: `DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`, or `PANIC`. The default value is `INFO`
	// +optional
	// +kubebuilder:default:="INFO"
	// +kubebuilder:validation:Enum=DEBUG;INFO;WARN;ERROR;FATAL;PANIC
	LogLevel string `json:"logLevel,omitempty"`
}

// Configuration for kube-rbac-proxy within the Che gateway pod.
type KubeRbacProxy struct {
	// The glog log level for the kube-rbac-proxy container within the gateway pod. Larger values represent a higher verbosity. The default value is `0`.
	// +optional
	// +kubebuilder:default:=0
	// +kubebuilder:validation:Minimum:=0
	LogLevel *int32 `json:"logLevel,omitempty"`
}

type AllowedSources struct {
	// The list of approved URLs for starting Cloud Development Environments (CDEs). CDEs can only be
	// initiated from these URLs. Wildcards `*` are supported in URLs, allowing flexible matching for
	// specific URL patterns. For instance, `https://example.com/*` would allow CDEs to be initiated
	// from any path within 'example.com'.
	// +optional
	Urls []string `json:"urls,omitempty"`
}

// GatewayPhase describes the different phases of the Che gateway lifecycle.
type GatewayPhase string

const (
	GatewayPhaseInitializing = "Initializing"
	GatewayPhaseEstablished  = "Established"
	GatewayPhaseInactive     = "Inactive"
)

// CheClusterPhase describes the different phases of the Che cluster lifecycle.
type CheClusterPhase string

const (
	ClusterPhaseActive          = "Active"
	ClusterPhaseInactive        = "Inactive"
	ClusterPhasePendingDeletion = "PendingDeletion"
	RollingUpdate               = "RollingUpdate"
)

// CheClusterStatus defines the observed state of Che installation.
type CheClusterStatus struct {
	// Specifies the current phase of the gateway deployment.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Gateway phase"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	GatewayPhase GatewayPhase `json:"gatewayPhase,omitempty"`
	// Currently installed Che version.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="displayName: Eclipse Che version"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	CheVersion string `json:"cheVersion"`
	// Public URL of the Che server.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Eclipse Che URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	CheURL string `json:"cheURL"`
	// Specifies the current phase of the Che deployment.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="ChePhase"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	ChePhase CheClusterPhase `json:"chePhase,omitempty"`
	// Deprecated the public URL of the internal devfile registry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Devfile registry URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	DevfileRegistryURL string `json:"devfileRegistryURL"`
	// The public URL of the internal plug-in registry.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Plugin registry URL"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:org.w3:link"
	PluginRegistryURL string `json:"pluginRegistryURL"`
	// A human readable message indicating details about why the Che deployment is in the current phase.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Message"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the Che deployment is in the current phase.
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=status
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Reason"
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors="urn:alm:descriptor:text"
	Reason string `json:"reason,omitempty"`
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
// `che`, `plugin-registry` that will contain the appropriate environment variables
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

	// Defines the observed state of Che installation.
	Status CheClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// The CheClusterList contains a list of CheClusters.
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

	return "<username>-" + defaults.GetCheFlavor()
}

func (c *CheCluster) GetIdentityToken() string {
	if len(c.Spec.Networking.Auth.IdentityToken) > 0 {
		return c.Spec.Networking.Auth.IdentityToken
	}

	if infrastructure.IsOpenShift() {
		return constants.AccessToken
	}
	return constants.IdToken
}

func (c *CheCluster) IsAccessTokenConfigured() bool {
	return c.GetIdentityToken() == constants.AccessToken
}

// IsContainerBuildCapabilitiesEnabled returns true if container build capabilities are enabled.
// If value is not set in the CheCluster CR, then the default value is used.
func (c *CheCluster) IsContainerBuildCapabilitiesEnabled() bool {
	disableContainerBuildCapabilitiesParsed, err := strconv.ParseBool(defaults.GetDevEnvironmentsDisableContainerBuildCapabilities())
	if err != nil {
		logger.Error(err, "Failed to parse disableContainerBuildCapabilities", "value", disableContainerBuildCapabilitiesParsed)
		return false
	}

	if c.Spec.DevEnvironments.DisableContainerBuildCapabilities != nil {
		disableContainerBuildCapabilitiesParsed = *c.Spec.DevEnvironments.DisableContainerBuildCapabilities
	}

	return !disableContainerBuildCapabilitiesParsed
}

func (c *CheCluster) IsOpenShiftSecurityContextConstraintSet() bool {
	return c.Spec.DevEnvironments.ContainerBuildConfiguration != nil && c.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint != ""
}

func (c *CheCluster) IsCheFlavor() bool {
	return defaults.GetCheFlavor() == constants.CheFlavor
}

// IsEmbeddedOpenVSXRegistryConfigured returns true if the Open VSX Registry is configured to be embedded
// only if only the `Spec.Components.PluginRegistry.OpenVSXURL` is empty.
func (c *CheCluster) IsEmbeddedOpenVSXRegistryConfigured() bool {
	if c.Spec.Components.PluginRegistry.OpenVSXURL != nil {
		return *c.Spec.Components.PluginRegistry.OpenVSXURL == ""
	}
	return defaults.GetPluginRegistryOpenVSXURL() == ""
}

func (c *CheCluster) IsInternalPluginRegistryDisabled() bool {
	return c.Spec.Components.PluginRegistry.DisableInternalRegistry || !c.IsEmbeddedOpenVSXRegistryConfigured()
}

// IsCheBeingInstalled returns true if the Che version is not set in the status.
// Basically it means that the Che is being installed since the Che version is set only after the installation.
func (c *CheCluster) IsCheBeingInstalled() bool {
	return c.Status.CheVersion == ""
}
