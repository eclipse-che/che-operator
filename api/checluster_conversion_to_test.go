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

package org

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	devfile "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev1 "github.com/eclipse-che/che-operator/api/v1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertToEmptyCheCluster(t *testing.T) {
	checlusterv1 := &chev1.CheCluster{}
	checlusterv2 := &chev2.CheCluster{}

	err := checlusterv1.ConvertTo(checlusterv2)
	assert.Nil(t, err)
}

func TestConvertToIngressOnOpenShift(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	checlusterv1 := &chev1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev1.CheClusterSpec{
			Server: chev1.CheClusterSpecServer{
				CheHost:          "CheHost",
				CheHostTLSSecret: "CheHostTLSSecret",
				CheServerIngress: chev1.IngressCustomSettings{
					Labels: "a=b,c=d",
				},
				CheServerRoute: chev1.RouteCustomSettings{
					Labels:      "a=b,c=d",
					Annotations: map[string]string{"a": "b", "c": "d"},
					Domain:      "CheServerRoute.Domain",
				},
			},
		},
	}

	checlusterv2 := &chev2.CheCluster{}
	err := checlusterv1.ConvertTo(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, checlusterv2.Spec.Networking.Annotations)
	assert.Equal(t, "CheServerRoute.Domain", checlusterv2.Spec.Networking.Domain)
	assert.Equal(t, "CheHost", checlusterv2.Spec.Networking.Hostname)
	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, checlusterv2.Spec.Networking.Labels)
	assert.Equal(t, "CheHostTLSSecret", checlusterv2.Spec.Networking.TlsSecretName)
}

func TestConvertToIngressOnK8s(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	checlusterv1 := &chev1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev1.CheClusterSpec{
			Server: chev1.CheClusterSpecServer{
				CheHost: "CheHost",
				CheServerIngress: chev1.IngressCustomSettings{
					Labels:      "a=b,c=d",
					Annotations: map[string]string{"a": "b", "c": "d"},
				},
			},
			K8s: chev1.CheClusterSpecK8SOnly{
				IngressDomain: "k8s.IngressDomain",
				IngressClass:  "k8s.IngressClass",
				TlsSecretName: "k8s.TlsSecretName",
			},
		},
	}

	checlusterv2 := &chev2.CheCluster{}
	err := checlusterv1.ConvertTo(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, map[string]string{"a": "b", "c": "d", "kubernetes.io/ingress.class": "k8s.IngressClass"}, checlusterv2.Spec.Networking.Annotations)
	assert.Equal(t, "k8s.IngressDomain", checlusterv2.Spec.Networking.Domain)
	assert.Equal(t, "CheHost", checlusterv2.Spec.Networking.Hostname)
	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, checlusterv2.Spec.Networking.Labels)
	assert.Equal(t, "k8s.TlsSecretName", checlusterv2.Spec.Networking.TlsSecretName)
}

func TestConvertToIngressOnK8sWithCheHostTLSSecret(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	checlusterv1 := &chev1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev1.CheClusterSpec{
			Server: chev1.CheClusterSpecServer{
				CheHost:          "CheHost",
				CheHostTLSSecret: "CheHostTLSSecret",
				CheServerIngress: chev1.IngressCustomSettings{
					Labels:      "a=b,c=d",
					Annotations: map[string]string{"a": "b", "c": "d"},
				},
			},
			K8s: chev1.CheClusterSpecK8SOnly{
				IngressDomain: "k8s.IngressDomain",
				IngressClass:  "k8s.IngressClass",
				TlsSecretName: "k8s.TlsSecretName",
			},
		},
	}

	checlusterv2 := &chev2.CheCluster{}
	err := checlusterv1.ConvertTo(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, map[string]string{"a": "b", "c": "d", "kubernetes.io/ingress.class": "k8s.IngressClass"}, checlusterv2.Spec.Networking.Annotations)
	assert.Equal(t, "k8s.IngressDomain", checlusterv2.Spec.Networking.Domain)
	assert.Equal(t, "CheHost", checlusterv2.Spec.Networking.Hostname)
	assert.Equal(t, map[string]string{"a": "b", "c": "d"}, checlusterv2.Spec.Networking.Labels)
	assert.Equal(t, "CheHostTLSSecret", checlusterv2.Spec.Networking.TlsSecretName)
}

func TestConvertTo(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	checlusterv1 := &chev1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev1.CheClusterSpec{
			Server: chev1.CheClusterSpecServer{
				AirGapContainerRegistryHostname:     "AirGapContainerRegistryHostname",
				AirGapContainerRegistryOrganization: "AirGapContainerRegistryOrganization",
				CheImage:                            "CheImage",
				CheImageTag:                         "CheImageTag",
				CheImagePullPolicy:                  "Always",
				CheLogLevel:                         "CheLogLevel",
				CheDebug:                            "true",
				CheClusterRoles:                     "CheClusterRoles_1,CheClusterRoles_2",
				CheWorkspaceClusterRole:             "ClusterRoles_1,ClusterRoles_2",
				WorkspaceNamespaceDefault:           "WorkspaceNamespaceDefault",
				AllowAutoProvisionUserNamespace:     pointer.BoolPtr(true),
				WorkspaceDefaultEditor:              "WorkspaceDefaultEditor",
				WorkspaceDefaultComponents: []devfile.Component{
					{
						Name: "universal-developer-image",
					},
				},
				ServerTrustStoreConfigMapName: "ServerTrustStoreConfigMapName",
				GitSelfSignedCert:             true,
				DashboardImage:                "DashboardImage",
				DashboardImagePullPolicy:      "Always",
				DashboardMemoryLimit:          "200Mi",
				DashboardMemoryRequest:        "100Mi",
				DashboardCpuLimit:             "2",
				DashboardCpuRequest:           "1",
				DevfileRegistryImage:          "DevfileRegistryImage",
				DevfileRegistryPullPolicy:     "Always",
				DevfileRegistryMemoryLimit:    "200Mi",
				DevfileRegistryMemoryRequest:  "100Mi",
				DevfileRegistryCpuLimit:       "2",
				DevfileRegistryCpuRequest:     "1",
				ExternalDevfileRegistry:       true,
				ExternalDevfileRegistries: []chev1.ExternalDevfileRegistries{
					{
						Url: "ExternalDevfileRegistries_1",
					},
					{
						Url: "ExternalDevfileRegistries_2",
					},
				},
				PluginRegistryUrl:                   "PluginRegistryUrl",
				PluginRegistryImage:                 "PluginRegistryImage",
				PluginRegistryPullPolicy:            "Always",
				PluginRegistryMemoryLimit:           "200Mi",
				PluginRegistryMemoryRequest:         "100Mi",
				PluginRegistryCpuLimit:              "2",
				PluginRegistryCpuRequest:            "1",
				ExternalPluginRegistry:              true,
				CustomCheProperties:                 map[string]string{"a": "b", "c": "d"},
				ProxyURL:                            "ProxyURL",
				ProxyPort:                           "ProxyPort",
				ProxySecret:                         "ProxySecret",
				NonProxyHosts:                       "NonProxyHosts_1|NonProxyHosts_2",
				ServerMemoryRequest:                 "100Mi",
				ServerMemoryLimit:                   "200Mi",
				ServerCpuLimit:                      "2",
				ServerCpuRequest:                    "1",
				SingleHostGatewayImage:              "SingleHostGatewayImage",
				SingleHostGatewayConfigSidecarImage: "SingleHostGatewayConfigSidecarImage",
				SingleHostGatewayConfigMapLabels:    map[string]string{"a": "b", "c": "d"},
				WorkspacesDefaultPlugins: []chev1.WorkspacesDefaultPlugins{
					{
						Editor:  "Editor",
						Plugins: []string{"Plugin_1,Plugin_2"},
					},
				},
				WorkspacePodNodeSelector: map[string]string{"a": "b", "c": "d"},
				WorkspacePodTolerations: []corev1.Toleration{
					{
						Key:      "Key",
						Operator: "Operator",
						Value:    "Value",
						Effect:   "Effect",
					},
				},
				CheServerEnv: []corev1.EnvVar{
					{
						Name:  "che-server-name",
						Value: "che-server-value",
					},
				},
				PluginRegistryEnv: []corev1.EnvVar{
					{
						Name:  "plugin-registry-name",
						Value: "plugin-registry-value",
					},
				},
				DevfileRegistryEnv: []corev1.EnvVar{
					{
						Name:  "devfile-registry-name",
						Value: "devfile-registry-value",
					},
				},
				DashboardEnv: []corev1.EnvVar{
					{
						Name:  "dashboard-name",
						Value: "dashboard-value",
					},
				},
				OpenVSXRegistryURL: pointer.StringPtr("open-vsx-registry"),
			},
			Auth: chev1.CheClusterSpecAuth{
				IdentityProviderURL:               "IdentityProviderURL",
				OAuthClientName:                   "OAuthClientName",
				OAuthSecret:                       "OAuthSecret",
				OAuthScope:                        "OAuthScope",
				IdentityToken:                     "IdentityToken",
				GatewayAuthenticationSidecarImage: "GatewayAuthenticationSidecarImage",
				GatewayAuthorizationSidecarImage:  "GatewayAuthorizationSidecarImage",
				GatewayConfigBumpEnv: []corev1.EnvVar{
					{
						Name:  "configbump-name",
						Value: "configbump-value",
					},
				},
				GatewayOAuthProxyEnv: []corev1.EnvVar{
					{
						Name:  "oauth-proxy-name",
						Value: "oauth-proxy-value",
					},
				},
				GatewayEnv: []corev1.EnvVar{
					{
						Name:  "gateway-name",
						Value: "gateway-value",
					},
				},
				GatewayKubeRbacProxyEnv: []corev1.EnvVar{
					{
						Name:  "kube-rbac-proxy-name",
						Value: "kube-rbac-proxy-value",
					},
				},
			},
			Storage: chev1.CheClusterSpecStorage{
				PvcStrategy:                             "PvcStrategy",
				PvcClaimSize:                            "WorkspacePvcClaimSize",
				WorkspacePVCStorageClassName:            "WorkspacePVCStorageClassName",
				PerWorkspaceStrategyPVCStorageClassName: "PerWorkspaceStrategyPVCStorageClassName",
				PerWorkspaceStrategyPvcClaimSize:        "PerWorkspaceStrategyPvcClaimSize",
			},
			Metrics: chev1.CheClusterSpecMetrics{
				Enable: true,
			},
			K8s: chev1.CheClusterSpecK8SOnly{
				SecurityContextFsGroup:   "64",
				SecurityContextRunAsUser: "65",
			},
			ImagePuller: chev1.CheClusterSpecImagePuller{
				Enable: true,
			},
			DevWorkspace: chev1.CheClusterSpecDevWorkspace{
				Enable:                          true,
				RunningLimit:                    "10",
				SecondsOfInactivityBeforeIdling: pointer.Int32Ptr(1800),
				SecondsOfRunBeforeIdling:        pointer.Int32Ptr(-1),
			},
			Dashboard: chev1.CheClusterSpecDashboard{
				Warning: "DashboardWarning",
			},
			GitServices: chev1.CheClusterGitServices{
				GitHub: []chev1.GitHubService{
					{
						SecretName: "github-secret-name",
						Endpoint:   "github-endpoint",
					},
				},
				GitLab: []chev1.GitLabService{
					{
						SecretName: "gitlab-secret-name",
						Endpoint:   "gitlab-endpoint",
					},
				},
				BitBucket: []chev1.BitBucketService{
					{
						SecretName: "bitbucket-secret-name",
						Endpoint:   "bitbucket-endpoint",
					},
				},
			}},
		Status: chev1.CheClusterStatus{
			CheClusterRunning:  "Available",
			CheVersion:         "CheVersion",
			CheURL:             "CheURL",
			Message:            "Message",
			Reason:             "Reason",
			HelpLink:           "HelpLink",
			DevfileRegistryURL: "DevfileRegistryURL",
			PluginRegistryURL:  "PluginRegistryURL",
		},
	}

	checlusterv2 := &chev2.CheCluster{}
	err := checlusterv1.ConvertTo(checlusterv2)
	assert.Nil(t, err)

	assert.Equal(t, checlusterv2.ObjectMeta.Name, "eclipse-che")
	assert.Equal(t, checlusterv2.ObjectMeta.Namespace, "eclipse-che")

	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[0].Name, constants.GatewayContainerName)
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[0].Image, "SingleHostGatewayImage")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[0].Env[0].Name, "gateway-name")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[0].Env[0].Value, "gateway-value")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[1].Name, constants.GatewayConfigSideCarContainerName)
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[1].Image, "SingleHostGatewayConfigSidecarImage")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[1].Env[0].Name, "configbump-name")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[1].Env[0].Value, "configbump-value")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[2].Name, constants.GatewayAuthenticationContainerName)
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[2].Image, "GatewayAuthenticationSidecarImage")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[2].Env[0].Name, "oauth-proxy-name")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[2].Env[0].Value, "oauth-proxy-value")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[3].Name, constants.GatewayAuthorizationContainerName)
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[3].Image, "GatewayAuthorizationSidecarImage")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[3].Env[0].Name, "kube-rbac-proxy-name")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.Deployment.Containers[3].Env[0].Value, "kube-rbac-proxy-value")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.Gateway.ConfigLabels, map[string]string{"a": "b", "c": "d"})
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.IdentityProviderURL, "IdentityProviderURL")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.OAuthClientName, "OAuthClientName")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.OAuthSecret, "OAuthSecret")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.OAuthScope, "OAuthScope")
	assert.Equal(t, checlusterv2.Spec.Networking.Auth.IdentityToken, "IdentityToken")

	assert.Equal(t, checlusterv2.Spec.ContainerRegistry.Hostname, "AirGapContainerRegistryHostname")
	assert.Equal(t, checlusterv2.Spec.ContainerRegistry.Organization, "AirGapContainerRegistryOrganization")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.ClusterRoles, []string{"CheClusterRoles_1", "CheClusterRoles_2"})
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.ExtraProperties, map[string]string{"a": "b", "c": "d"})
	assert.True(t, *checlusterv2.Spec.Components.CheServer.Debug)
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Image, "CheImage:CheImageTag")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Name, defaults.GetCheFlavor())
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Env[0].Name, "che-server-name")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Env[0].Value, "che-server-value")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].ImagePullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits.Cpu, resource.MustParse("2"))
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits.Memory, resource.MustParse("200Mi"))
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests.Cpu, resource.MustParse("1"))
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests.Memory, resource.MustParse("100Mi"))
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.SecurityContext.FsGroup, int64(64))
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.SecurityContext.RunAsUser, int64(65))
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.LogLevel, "CheLogLevel")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Proxy.CredentialsSecretName, "ProxySecret")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Proxy.NonProxyHosts, []string{"NonProxyHosts_1", "NonProxyHosts_2"})
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Proxy.Port, "ProxyPort")
	assert.Equal(t, checlusterv2.Spec.Components.CheServer.Proxy.Url, "ProxyURL")

	assert.Equal(t, checlusterv2.Spec.DevEnvironments.TrustedCerts.GitTrustedCertsConfigMapName, "che-git-self-signed-cert")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.DefaultNamespace.Template, "WorkspaceNamespaceDefault")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.DefaultNamespace.AutoProvision, pointer.BoolPtr(true))
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.DefaultEditor, "WorkspaceDefaultEditor")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.DefaultComponents, []devfile.Component{{Name: "universal-developer-image"}})
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.NodeSelector, map[string]string{"a": "b", "c": "d"})
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.Tolerations, []corev1.Toleration{{
		Key:      "Key",
		Operator: "Operator",
		Value:    "Value",
		Effect:   "Effect",
	}})
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.DefaultPlugins, []chev2.WorkspaceDefaultPlugins{{
		Editor:  "Editor",
		Plugins: []string{"Plugin_1,Plugin_2"},
	}})
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.User.ClusterRoles, []string{"CheClusterRoles_1", "CheClusterRoles_2"})

	assert.Equal(t, checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Name, defaults.GetCheFlavor()+"-dashboard")
	assert.Equal(t, checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Image, "DashboardImage")
	assert.Equal(t, checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Env[0].Name, "dashboard-name")
	assert.Equal(t, checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Env[0].Value, "dashboard-value")
	assert.Equal(t, checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].ImagePullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, *checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Limits.Cpu, resource.MustParse("2"))
	assert.Equal(t, *checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Limits.Memory, resource.MustParse("200Mi"))
	assert.Equal(t, *checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Requests.Cpu, resource.MustParse("1"))
	assert.Equal(t, *checlusterv2.Spec.Components.Dashboard.Deployment.Containers[0].Resources.Requests.Memory, resource.MustParse("100Mi"))
	assert.Equal(t, *checlusterv2.Spec.Components.Dashboard.Deployment.SecurityContext.FsGroup, int64(64))
	assert.Equal(t, *checlusterv2.Spec.Components.Dashboard.Deployment.SecurityContext.RunAsUser, int64(65))
	assert.Equal(t, checlusterv2.Spec.Components.Dashboard.HeaderMessage.Text, "DashboardWarning")
	assert.True(t, checlusterv2.Spec.Components.Dashboard.HeaderMessage.Show)

	assert.Equal(t, checlusterv2.Spec.Components.ImagePuller.Enable, true)
	assert.Equal(t, checlusterv2.Spec.Components.Metrics.Enable, true)

	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Name, constants.DevfileRegistryName)
	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Image, "DevfileRegistryImage")
	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Env[0].Name, "devfile-registry-name")
	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Env[0].Value, "devfile-registry-value")
	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].ImagePullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, *checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Limits.Cpu, resource.MustParse("2"))
	assert.Equal(t, *checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Limits.Memory, resource.MustParse("200Mi"))
	assert.Equal(t, *checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Requests.Cpu, resource.MustParse("1"))
	assert.Equal(t, *checlusterv2.Spec.Components.DevfileRegistry.Deployment.Containers[0].Resources.Requests.Memory, resource.MustParse("100Mi"))
	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.DisableInternalRegistry, true)
	assert.Equal(t, checlusterv2.Spec.Components.DevfileRegistry.ExternalDevfileRegistries, []chev2.ExternalDevfileRegistry{
		{
			Url: "ExternalDevfileRegistries_1",
		},
		{
			Url: "ExternalDevfileRegistries_2",
		}})

	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Name, constants.PluginRegistryName)
	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Image, "PluginRegistryImage")
	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Env[0].Name, "plugin-registry-name")
	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Env[0].Value, "plugin-registry-value")
	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].ImagePullPolicy, corev1.PullPolicy("Always"))
	assert.Equal(t, *checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Limits.Cpu, resource.MustParse("2"))
	assert.Equal(t, *checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Limits.Memory, resource.MustParse("200Mi"))
	assert.Equal(t, *checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Requests.Cpu, resource.MustParse("1"))
	assert.Equal(t, *checlusterv2.Spec.Components.PluginRegistry.Deployment.Containers[0].Resources.Requests.Memory, resource.MustParse("100Mi"))
	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.DisableInternalRegistry, true)
	assert.Equal(t, *checlusterv2.Spec.Components.PluginRegistry.OpenVSXURL, "open-vsx-registry")
	assert.Equal(t, checlusterv2.Spec.Components.PluginRegistry.ExternalPluginRegistries, []chev2.ExternalPluginRegistry{{Url: "PluginRegistryUrl"}})

	assert.Equal(t, checlusterv2.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.ClaimSize, "WorkspacePvcClaimSize")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.Storage.PerUserStrategyPvcConfig.StorageClass, "WorkspacePVCStorageClassName")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.ClaimSize, "PerWorkspaceStrategyPvcClaimSize")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.Storage.PerWorkspaceStrategyPvcConfig.StorageClass, "PerWorkspaceStrategyPVCStorageClassName")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.Storage.PvcStrategy, "PvcStrategy")
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.SecondsOfInactivityBeforeIdling, pointer.Int32Ptr(1800))
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.SecondsOfRunBeforeIdling, pointer.Int32Ptr(-1))
	assert.Equal(t, checlusterv2.Spec.DevEnvironments.MaxNumberOfRunningWorkspacesPerUser, pointer.Int64Ptr(10))

	assert.Equal(t, checlusterv2.Status.CheURL, "CheURL")
	assert.Equal(t, checlusterv2.Status.CheVersion, "CheVersion")
	assert.Equal(t, checlusterv2.Status.DevfileRegistryURL, "DevfileRegistryURL")
	assert.Equal(t, checlusterv2.Status.Message, "Message")
	assert.Equal(t, checlusterv2.Status.ChePhase, chev2.CheClusterPhase("Active"))
	assert.Equal(t, checlusterv2.Status.PluginRegistryURL, "PluginRegistryURL")
	assert.Equal(t, checlusterv2.Status.Reason, "Reason")

	assert.Equal(t, checlusterv2.Spec.GitServices.GitHub[0].SecretName, "github-secret-name")
	assert.Equal(t, checlusterv2.Spec.GitServices.GitLab[0].SecretName, "gitlab-secret-name")
	assert.Equal(t, checlusterv2.Spec.GitServices.BitBucket[0].SecretName, "bitbucket-secret-name")
}

func TestShouldConvertToWhenOnlyMemoryResourceSpecified(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	checlusterv1 := &chev1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev1.CheClusterSpec{
			Server: chev1.CheClusterSpecServer{
				ServerMemoryLimit:   "10Gi",
				ServerMemoryRequest: "5Gi",
			},
		},
	}

	checlusterv2 := &chev2.CheCluster{}
	err := checlusterv1.ConvertTo(checlusterv2)
	assert.Nil(t, err)

	assert.Nil(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits.Cpu)
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Limits.Memory, resource.MustParse("10Gi"))
	assert.Nil(t, checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests.Cpu)
	assert.Equal(t, *checlusterv2.Spec.Components.CheServer.Deployment.Containers[0].Resources.Requests.Memory, resource.MustParse("5Gi"))
}
