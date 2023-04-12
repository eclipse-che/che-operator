//
// Copyright (c) 2019-2022 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package org

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"

	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev1 "github.com/eclipse-che/che-operator/api/v1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

func TestConvertFromEmptyCheCluster(t *testing.T) {
	checlusterv1 := &chev1.CheCluster{}
	checlusterv2 := &chev2.CheCluster{}

	err := checlusterv1.ConvertFrom(checlusterv2)
	assert.Nil(t, err)
}

func TestConvertFromIngressOnK8s(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	checlusterv2 := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Labels:        map[string]string{"a": "b", "c": "d"},
				Annotations:   map[string]string{"a": "b", "c": "d", "kubernetes.io/ingress.class": "nginx"},
				Domain:        "Domain",
				Hostname:      "Hostname",
				TlsSecretName: "tlsSecret",
			},
		},
	}

	checlusterv1 := &chev1.CheCluster{}
	err := checlusterv1.ConvertFrom(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, checlusterv1.Spec.Server.CheServerIngress.Annotations)
	assert.Equal(t, "Domain", checlusterv1.Spec.K8s.IngressDomain)
	assert.Equal(t, "nginx", checlusterv1.Spec.K8s.IngressClass)
	assert.Equal(t, "Hostname", checlusterv1.Spec.Server.CheHost)
	assert.Equal(t, "a=b,c=d", checlusterv1.Spec.Server.CheServerIngress.Labels)
	assert.Equal(t, "tlsSecret", checlusterv1.Spec.K8s.TlsSecretName)
}

func TestConvertFromIngressOnOpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	checlusterv2 := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Labels:        map[string]string{"a": "b", "c": "d"},
				Annotations:   map[string]string{"a": "b", "c": "d"},
				Domain:        "Domain",
				Hostname:      "Hostname",
				TlsSecretName: "tlsSecret",
			},
		},
	}

	checlusterv1 := &chev1.CheCluster{}
	err := checlusterv1.ConvertFrom(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, checlusterv1.Spec.Server.CheServerRoute.Annotations)
	assert.Equal(t, "Domain", checlusterv1.Spec.Server.CheServerRoute.Domain)
	assert.Equal(t, "Hostname", checlusterv1.Spec.Server.CheHost)
	assert.Equal(t, "a=b,c=d", checlusterv1.Spec.Server.CheServerRoute.Labels)
	assert.Equal(t, "tlsSecret", checlusterv1.Spec.Server.CheHostTLSSecret)
}

func TestConvertFrom(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	memoryRequest := resource.MustParse("128Mi")
	cpuRequest := resource.MustParse("1")
	memoryLimit := resource.MustParse("228Mi")
	cpuLimit := resource.MustParse("2")

	checlusterv2 := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				Metrics: chev2.ServerMetrics{
					Enable: true,
				},
				PluginRegistry: chev2.PluginRegistry{
					Deployment: &chev2.Deployment{
						Containers: []chev2.Container{
							{
								Image:           "PluginRegistryImage",
								ImagePullPolicy: corev1.PullAlways,
								Resources: &chev2.ResourceRequirements{
									Requests: &chev2.ResourceList{
										Memory: &memoryRequest,
										Cpu:    &cpuRequest,
									},
									Limits: &chev2.ResourceList{
										Memory: &memoryLimit,
										Cpu:    &cpuLimit,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "plugin-registry-name",
										Value: "plugin-registry-value",
									},
								},
							},
						},
						SecurityContext: &chev2.PodSecurityContext{
							RunAsUser: pointer.Int64Ptr(64),
							FsGroup:   pointer.Int64Ptr(65),
						},
					},
					DisableInternalRegistry: true,
					ExternalPluginRegistries: []chev2.ExternalPluginRegistry{
						{
							Url: "ExternalPluginRegistries_1",
						},
						{
							Url: "ExternalPluginRegistries_2",
						},
					},
					OpenVSXURL: pointer.StringPtr("open-vsx-registry"),
				},
				DevfileRegistry: chev2.DevfileRegistry{
					Deployment: &chev2.Deployment{
						Containers: []chev2.Container{
							{
								Image:           "DevfileRegistryImage",
								ImagePullPolicy: corev1.PullAlways,
								Resources: &chev2.ResourceRequirements{
									Requests: &chev2.ResourceList{
										Memory: &memoryRequest,
										Cpu:    &cpuRequest,
									},
									Limits: &chev2.ResourceList{
										Memory: &memoryLimit,
										Cpu:    &cpuLimit,
									}},
								Env: []corev1.EnvVar{
									{
										Name:  "devfile-registry-name",
										Value: "devfile-registry-value",
									},
								},
							},
						},
						SecurityContext: &chev2.PodSecurityContext{
							RunAsUser: pointer.Int64Ptr(64),
							FsGroup:   pointer.Int64Ptr(65),
						},
					},
					DisableInternalRegistry: true,
					ExternalDevfileRegistries: []chev2.ExternalDevfileRegistry{
						{
							Url: "ExternalDevfileRegistries",
						},
					},
				},
				Dashboard: chev2.Dashboard{
					Deployment: &chev2.Deployment{
						Containers: []chev2.Container{
							{
								Image:           "DashboardImage",
								ImagePullPolicy: corev1.PullAlways,
								Resources: &chev2.ResourceRequirements{
									Requests: &chev2.ResourceList{
										Memory: &memoryRequest,
										Cpu:    &cpuRequest,
									},
									Limits: &chev2.ResourceList{
										Memory: &memoryLimit,
										Cpu:    &cpuLimit,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "dashboard-name",
										Value: "dashboard-value",
									},
								},
							},
						},
						SecurityContext: &chev2.PodSecurityContext{
							RunAsUser: pointer.Int64Ptr(64),
							FsGroup:   pointer.Int64Ptr(65),
						},
					},
					HeaderMessage: &chev2.DashboardHeaderMessage{
						Show: true,
						Text: "DashboardWarning",
					},
				},
				ImagePuller: chev2.ImagePuller{
					Enable: true,
				},
				CheServer: chev2.CheServer{
					ExtraProperties: map[string]string{"a": "b", "c": "d"},
					Deployment: &chev2.Deployment{
						Containers: []chev2.Container{
							{
								Image:           "ServerImage:ServerTag",
								ImagePullPolicy: corev1.PullAlways,
								Resources: &chev2.ResourceRequirements{
									Requests: &chev2.ResourceList{
										Memory: &memoryRequest,
										Cpu:    &cpuRequest,
									},
									Limits: &chev2.ResourceList{
										Memory: &memoryLimit,
										Cpu:    &cpuLimit,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "che-server-name",
										Value: "che-server-value",
									},
								}},
						},
						SecurityContext: &chev2.PodSecurityContext{
							RunAsUser: pointer.Int64Ptr(64),
							FsGroup:   pointer.Int64Ptr(65),
						},
					},
					LogLevel:     "LogLevel",
					Debug:        pointer.BoolPtr(true),
					ClusterRoles: []string{"ClusterRoles_1", "ClusterRoles_2"},
					Proxy: &chev2.Proxy{
						Url:                   "ProxyUrl",
						Port:                  "ProxyPort",
						NonProxyHosts:         []string{"NonProxyHosts_1", "NonProxyHosts_2"},
						CredentialsSecretName: "ProxyCredentialsSecretName",
					},
				},
			},
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					IdentityProviderURL: "IdentityProviderURL",
					OAuthClientName:     "OAuthClientName",
					OAuthSecret:         "OAuthSecret",
					OAuthScope:          "OAuthScope",
					IdentityToken:       "IdentityToken",
					Gateway: chev2.Gateway{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:  "gateway",
									Image: "GatewayImage",
									Env: []corev1.EnvVar{
										{
											Name:  "gateway-name",
											Value: "gateway-value",
										},
									},
								},
								{
									Name:  "configbump",
									Image: "ConfigSidecarImage",
									Env: []corev1.EnvVar{
										{
											Name:  "configbump-name",
											Value: "configbump-value",
										},
									},
								},
								{
									Name:  "oauth-proxy",
									Image: "AuthenticationSidecarImage",
									Env: []corev1.EnvVar{
										{
											Name:  "oauth-proxy-name",
											Value: "oauth-proxy-value",
										},
									},
								},
								{
									Name:  "kube-rbac-proxy",
									Image: "AuthorizationSidecarImage",
									Env: []corev1.EnvVar{
										{
											Name:  "kube-rbac-proxy-name",
											Value: "kube-rbac-proxy-value",
										},
									},
								},
							},
						},
						ConfigLabels: map[string]string{"a": "b", "c": "d"},
					},
				},
			},
			DevEnvironments: chev2.CheClusterDevEnvironments{
				DefaultNamespace: chev2.DefaultNamespace{
					Template:      "WorkspaceNamespaceName",
					AutoProvision: pointer.BoolPtr(true),
				},
				TrustedCerts: &chev2.TrustedCerts{
					GitTrustedCertsConfigMapName: "che-git-self-signed-cert",
				},
				Storage: chev2.WorkspaceStorage{
					PerUserStrategyPvcConfig: &chev2.PVC{
						ClaimSize:    "StorageClaimSize",
						StorageClass: "StorageClass",
					},
					PerWorkspaceStrategyPvcConfig: &chev2.PVC{
						ClaimSize:    "PerWorkspaceStorageClaimSize",
						StorageClass: "PerWorkspaceStorageClass",
					},
					PvcStrategy: "PvcStrategy",
				},
				DefaultPlugins: []chev2.WorkspaceDefaultPlugins{
					{
						Editor:  "Editor",
						Plugins: []string{"Plugins_1", "Plugins_2"},
					},
				},
				DefaultEditor: "DefaultEditor",
				DefaultComponents: []devfile.Component{
					{
						Name: "universal-developer-image",
					},
				},
				NodeSelector: map[string]string{"a": "b", "c": "d"},
				Tolerations: []corev1.Toleration{{
					Key:      "Key",
					Operator: "Operator",
					Value:    "Value",
					Effect:   "Effect",
				}},
				SecondsOfInactivityBeforeIdling:     pointer.Int32Ptr(1800),
				SecondsOfRunBeforeIdling:            pointer.Int32Ptr(-1),
				MaxNumberOfRunningWorkspacesPerUser: pointer.Int64Ptr(10),
			},
			ContainerRegistry: chev2.CheClusterContainerRegistry{
				Hostname:     "AirGapContainerRegistryHostname",
				Organization: "AirGapContainerRegistryOrganization",
			},
			GitServices: chev2.CheClusterGitServices{
				GitHub: []chev2.GitHubService{
					{
						SecretName: "github-secret-name",
					},
				},
				GitLab: []chev2.GitLabService{
					{
						SecretName: "gitlab-secret-name",
					},
				},
				BitBucket: []chev2.BitBucketService{
					{
						SecretName: "bitbucket-secret-name",
					},
				},
			},
		},
		Status: chev2.CheClusterStatus{
			CheVersion:         "CheVersion",
			CheURL:             "CheURL",
			DevfileRegistryURL: "DevfileRegistryURL",
			PluginRegistryURL:  "PluginRegistryURL",
			ChePhase:           "Active",
			Message:            "Message",
			Reason:             "Reason",
		},
	}

	checlusterv1 := &chev1.CheCluster{}
	err := checlusterv1.ConvertFrom(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, checlusterv1.ObjectMeta.Name, "eclipse-che")
	assert.Equal(t, checlusterv1.ObjectMeta.Namespace, "eclipse-che")

	assert.Equal(t, checlusterv1.Status.CheClusterRunning, "Available")
	assert.Equal(t, checlusterv1.Status.CheURL, "CheURL")
	assert.Equal(t, checlusterv1.Status.CheVersion, "CheVersion")
	assert.Equal(t, checlusterv1.Status.DevfileRegistryURL, "DevfileRegistryURL")
	assert.Equal(t, checlusterv1.Status.Message, "Message")
	assert.Equal(t, checlusterv1.Status.PluginRegistryURL, "PluginRegistryURL")
	assert.Equal(t, checlusterv1.Status.Reason, "Reason")
	assert.Equal(t, checlusterv1.Status.GitServerTLSCertificateConfigMapName, "che-git-self-signed-cert")

	assert.Equal(t, checlusterv1.Spec.Auth.GatewayAuthenticationSidecarImage, "AuthenticationSidecarImage")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayAuthorizationSidecarImage, "AuthorizationSidecarImage")
	assert.Equal(t, checlusterv1.Spec.Auth.IdentityProviderURL, "IdentityProviderURL")
	assert.Equal(t, checlusterv1.Spec.Auth.OAuthClientName, "OAuthClientName")
	assert.Equal(t, checlusterv1.Spec.Auth.OAuthSecret, "OAuthSecret")
	assert.Equal(t, checlusterv1.Spec.Auth.OAuthScope, "OAuthScope")
	assert.Equal(t, checlusterv1.Spec.Auth.IdentityToken, "IdentityToken")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayEnv[0].Name, "gateway-name")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayEnv[0].Value, "gateway-value")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayKubeRbacProxyEnv[0].Name, "kube-rbac-proxy-name")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayKubeRbacProxyEnv[0].Value, "kube-rbac-proxy-value")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayOAuthProxyEnv[0].Name, "oauth-proxy-name")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayOAuthProxyEnv[0].Value, "oauth-proxy-value")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayConfigBumpEnv[0].Name, "configbump-name")
	assert.Equal(t, checlusterv1.Spec.Auth.GatewayConfigBumpEnv[0].Value, "configbump-value")

	assert.Equal(t, checlusterv1.Spec.DevWorkspace.RunningLimit, "10")
	assert.Equal(t, checlusterv1.Spec.DevWorkspace.SecondsOfInactivityBeforeIdling, pointer.Int32Ptr(1800))
	assert.Equal(t, checlusterv1.Spec.DevWorkspace.SecondsOfRunBeforeIdling, pointer.Int32Ptr(-1))
	assert.True(t, checlusterv1.Spec.DevWorkspace.Enable)

	assert.Equal(t, checlusterv1.Spec.Dashboard.Warning, "DashboardWarning")

	assert.Equal(t, checlusterv1.Spec.ImagePuller.Enable, true)
	assert.Equal(t, checlusterv1.Spec.Metrics.Enable, true)

	assert.Equal(t, checlusterv1.Spec.Server.CheServerEnv[0].Name, "che-server-name")
	assert.Equal(t, checlusterv1.Spec.Server.CheServerEnv[0].Value, "che-server-value")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryEnv[0].Name, "plugin-registry-name")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryEnv[0].Value, "plugin-registry-value")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryEnv[0].Name, "devfile-registry-name")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryEnv[0].Value, "devfile-registry-value")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardEnv[0].Name, "dashboard-name")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardEnv[0].Value, "dashboard-value")
	assert.Equal(t, checlusterv1.Spec.Server.AirGapContainerRegistryHostname, "AirGapContainerRegistryHostname")
	assert.Equal(t, checlusterv1.Spec.Server.AirGapContainerRegistryOrganization, "AirGapContainerRegistryOrganization")
	assert.Equal(t, checlusterv1.Spec.Server.CheClusterRoles, "ClusterRoles_1,ClusterRoles_2")
	assert.Equal(t, checlusterv1.Spec.Server.CheDebug, "true")
	assert.Equal(t, checlusterv1.Spec.Server.CheImage, "ServerImage")
	assert.Equal(t, checlusterv1.Spec.Server.CheImagePullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, checlusterv1.Spec.Server.CheImageTag, "ServerTag")
	assert.Equal(t, checlusterv1.Spec.Server.CheLogLevel, "LogLevel")
	assert.Equal(t, checlusterv1.Spec.Server.CustomCheProperties, map[string]string{"a": "b", "c": "d"})
	assert.Equal(t, checlusterv1.Spec.Server.DashboardCpuLimit, "2")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardCpuRequest, "1")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardImage, "DashboardImage")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardImagePullPolicy, "Always")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardMemoryLimit, "228Mi")
	assert.Equal(t, checlusterv1.Spec.Server.DashboardMemoryRequest, "128Mi")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryCpuLimit, "2")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryCpuRequest, "1")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryImage, "DevfileRegistryImage")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryMemoryLimit, "228Mi")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryMemoryRequest, "128Mi")
	assert.Equal(t, checlusterv1.Spec.Server.DevfileRegistryPullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, checlusterv1.Spec.Server.ExternalDevfileRegistries, []chev1.ExternalDevfileRegistries{{Url: "ExternalDevfileRegistries"}})
	assert.Equal(t, checlusterv1.Spec.Server.ExternalDevfileRegistry, true)
	assert.Equal(t, checlusterv1.Spec.Server.ExternalPluginRegistry, true)
	assert.Equal(t, checlusterv1.Spec.Server.GitSelfSignedCert, true)
	assert.Equal(t, checlusterv1.Spec.Server.NonProxyHosts, "NonProxyHosts_1|NonProxyHosts_2")
	assert.Equal(t, *checlusterv1.Spec.Server.OpenVSXRegistryURL, "open-vsx-registry")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryCpuLimit, "2")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryCpuRequest, "1")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryImage, "PluginRegistryImage")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryMemoryLimit, "228Mi")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryMemoryRequest, "128Mi")
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryPullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, checlusterv1.Spec.Server.PluginRegistryUrl, "ExternalPluginRegistries_1")
	assert.Equal(t, checlusterv1.Spec.Server.ProxyPort, "ProxyPort")
	assert.Equal(t, checlusterv1.Spec.Server.ProxySecret, "ProxyCredentialsSecretName")
	assert.Equal(t, checlusterv1.Spec.Server.ProxyURL, "ProxyUrl")
	assert.Equal(t, checlusterv1.Spec.Server.ServerCpuLimit, "2")
	assert.Equal(t, checlusterv1.Spec.Server.ServerCpuRequest, "1")
	assert.Equal(t, checlusterv1.Spec.Server.ServerMemoryLimit, "228Mi")
	assert.Equal(t, checlusterv1.Spec.Server.ServerMemoryRequest, "128Mi")
	assert.Equal(t, checlusterv1.Spec.Server.SingleHostGatewayConfigMapLabels, labels.Set{"a": "b", "c": "d"})
	assert.Equal(t, checlusterv1.Spec.Server.SingleHostGatewayConfigSidecarImage, "ConfigSidecarImage")
	assert.Equal(t, checlusterv1.Spec.Server.SingleHostGatewayImage, "GatewayImage")
	assert.Equal(t, checlusterv1.Spec.Server.WorkspaceNamespaceDefault, "WorkspaceNamespaceName")
	assert.Equal(t, checlusterv1.Spec.Server.AllowAutoProvisionUserNamespace, pointer.BoolPtr(true))
	assert.Equal(t, checlusterv1.Spec.Server.WorkspaceDefaultEditor, "DefaultEditor")
	assert.Equal(t, checlusterv1.Spec.Server.WorkspaceDefaultComponents, []devfile.Component{{Name: "universal-developer-image"}})
	assert.Equal(t, checlusterv1.Spec.Server.WorkspacePodNodeSelector, map[string]string{"a": "b", "c": "d"})
	assert.Equal(t, checlusterv1.Spec.Server.WorkspacePodTolerations, []corev1.Toleration{
		{
			Key:      "Key",
			Operator: "Operator",
			Value:    "Value",
			Effect:   "Effect",
		},
	})
	assert.Equal(t, checlusterv1.Spec.Server.WorkspacesDefaultPlugins, []chev1.WorkspacesDefaultPlugins{{Editor: "Editor", Plugins: []string{"Plugins_1", "Plugins_2"}}})

	assert.Equal(t, checlusterv1.Spec.Storage.PvcStrategy, "PvcStrategy")
	assert.Equal(t, checlusterv1.Spec.Storage.PvcClaimSize, "StorageClaimSize")
	assert.Equal(t, checlusterv1.Spec.Storage.WorkspacePVCStorageClassName, "StorageClass")
	assert.Equal(t, checlusterv1.Spec.Storage.PerWorkspaceStrategyPvcClaimSize, "PerWorkspaceStorageClaimSize")
	assert.Equal(t, checlusterv1.Spec.Storage.PerWorkspaceStrategyPVCStorageClassName, "PerWorkspaceStorageClass")

	assert.Equal(t, checlusterv1.Spec.GitServices.GitHub[0].SecretName, "github-secret-name")
	assert.Equal(t, checlusterv1.Spec.GitServices.GitLab[0].SecretName, "gitlab-secret-name")
	assert.Equal(t, checlusterv1.Spec.GitServices.BitBucket[0].SecretName, "bitbucket-secret-name")
}

func TestShouldConvertFromWhenOnlyMemoryResourceSpecified(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	memoryRequest := resource.MustParse("128Mi")
	memoryLimit := resource.MustParse("228Mi")

	checlusterv2 := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					Deployment: &chev2.Deployment{
						Containers: []chev2.Container{
							{
								Resources: &chev2.ResourceRequirements{
									Requests: &chev2.ResourceList{
										Memory: &memoryRequest,
									},
									Limits: &chev2.ResourceList{
										Memory: &memoryLimit,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	checlusterv1 := &chev1.CheCluster{}
	err := checlusterv1.ConvertFrom(checlusterv2)
	assert.Nil(t, err)

	assert.Empty(t, checlusterv1.Spec.Server.ServerCpuLimit)
	assert.Empty(t, checlusterv1.Spec.Server.ServerCpuRequest)
	assert.Equal(t, checlusterv1.Spec.Server.ServerMemoryLimit, "228Mi")
	assert.Equal(t, checlusterv1.Spec.Server.ServerMemoryRequest, "128Mi")
}
