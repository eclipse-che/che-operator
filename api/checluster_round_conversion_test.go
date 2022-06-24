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
	"context"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/stretchr/testify/assert"
)

func TestRoundConvertEmptyCheClusterV2(t *testing.T) {
	f := func() {
		checlusterv2Orignal := &chev2.CheCluster{}

		checlusterv1 := &chev1.CheCluster{}
		checlusterv2 := &chev2.CheCluster{}

		err := checlusterv1.ConvertFrom(checlusterv2Orignal)
		assert.Nil(t, err)

		err = checlusterv1.ConvertTo(checlusterv2)
		assert.Nil(t, err)

		assert.Equal(t, checlusterv2Orignal, checlusterv2)
	}

	onKubernetes(f)
	onOpenShift(f)
}

func TestRoundConvertEmptyCheClusterV1(t *testing.T) {
	f := func() {
		checlusterv1Orignal := &chev1.CheCluster{
			Spec: chev1.CheClusterSpec{
				DevWorkspace: chev1.CheClusterSpecDevWorkspace{
					Enable: true,
				},
			},
		}

		checlusterv1 := &chev1.CheCluster{}
		checlusterv2 := &chev2.CheCluster{}

		err := checlusterv1Orignal.ConvertTo(checlusterv2)
		assert.Nil(t, err)

		err = checlusterv1.ConvertFrom(checlusterv2)
		assert.Nil(t, err)

		assert.Equal(t, checlusterv1Orignal, checlusterv1)
	}

	onKubernetes(f)
	onOpenShift(f)
}

func TestRoundConvertCheClusterV2(t *testing.T) {
	f := func() {
		checlusterv2Original := &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					Metrics: chev2.ServerMetrics{
						Enable: true,
					},
					Database: chev2.Database{
						ExternalDb: true,
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:            "postgres",
									Image:           "DatabaseImage",
									ImagePullPolicy: corev1.PullAlways,
									Resources: &chev2.ResourceRequirements{
										Requests: &chev2.ResourceList{
											Memory: resource.MustParse("148Mi"),
											Cpu:    resource.MustParse("1"),
										},
										Limits: &chev2.ResourceList{
											Memory: resource.MustParse("228Mi"),
											Cpu:    resource.MustParse("2"),
										},
									},
								},
							},
						},
						PostgresHostName:      "PostgresHostName",
						PostgresPort:          "PostgresPort",
						PostgresDb:            "PostgresDb",
						CredentialsSecretName: "DatabaseCredentialsSecretName",
						Pvc: &chev2.PVC{
							ClaimSize:    "DatabaseClaimSize",
							StorageClass: "DatabaseStorageClass",
						},
					},
					PluginRegistry: chev2.PluginRegistry{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:            "plugin-registry",
									Image:           "PluginRegistryImage",
									ImagePullPolicy: corev1.PullAlways,
									Resources: &chev2.ResourceRequirements{
										Requests: &chev2.ResourceList{
											Memory: resource.MustParse("148Mi"),
											Cpu:    resource.MustParse("1"),
										},
										Limits: &chev2.ResourceList{
											Memory: resource.MustParse("228Mi"),
											Cpu:    resource.MustParse("2"),
										},
									},
								},
							},
						},
						DisableInternalRegistry: true,
						ExternalPluginRegistries: []chev2.ExternalPluginRegistry{
							{
								Url: "ExternalPluginRegistries_1",
							},
						},
					},
					DevfileRegistry: chev2.DevfileRegistry{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:            "devfile-registry",
									Image:           "DevfileRegistryImage",
									ImagePullPolicy: corev1.PullAlways,
									Resources: &chev2.ResourceRequirements{
										Requests: &chev2.ResourceList{
											Memory: resource.MustParse("148Mi"),
											Cpu:    resource.MustParse("1"),
										},
										Limits: &chev2.ResourceList{
											Memory: resource.MustParse("228Mi"),
											Cpu:    resource.MustParse("2"),
										},
									},
								},
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
									Name:            defaults.GetCheFlavor() + "-dashboard",
									Image:           "DashboardImage",
									ImagePullPolicy: corev1.PullAlways,
									Resources: &chev2.ResourceRequirements{
										Requests: &chev2.ResourceList{
											Memory: resource.MustParse("148Mi"),
											Cpu:    resource.MustParse("1"),
										},
										Limits: &chev2.ResourceList{
											Memory: resource.MustParse("228Mi"),
											Cpu:    resource.MustParse("2"),
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
									Name:            defaults.GetCheFlavor(),
									Image:           "ServerImage:ServerTag",
									ImagePullPolicy: corev1.PullAlways,
									Resources: &chev2.ResourceRequirements{
										Requests: &chev2.ResourceList{
											Memory: resource.MustParse("148Mi"),
											Cpu:    resource.MustParse("1"),
										},
										Limits: &chev2.ResourceList{
											Memory: resource.MustParse("228Mi"),
											Cpu:    resource.MustParse("2"),
										},
									},
								},
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
					DevWorkspace: chev2.DevWorkspace{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:  "devworkspace-controller",
									Image: "DevWorkspaceImage",
								},
							},
						},
						RunningLimit: "RunningLimit",
					},
				},
				Networking: chev2.CheClusterSpecNetworking{
					TlsSecretName: "che-tls",
					Domain:        "domain",
					Hostname:      "hostname",
					Labels: map[string]string{
						"label": "value",
					},
					Annotations: map[string]string{
						"kubernetes.io/ingress.class": "nginx",
					},
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
									},
									{
										Name:  "configbump",
										Image: "ConfigSidecarImage",
									},
									{
										Name:  "oauth-proxy",
										Image: "AuthenticationSidecarImage",
									},
									{
										Name:  "kube-rbac-proxy",
										Image: "AuthorizationSidecarImage",
									},
								},
							},
							ConfigLabels: map[string]string{"a": "b", "c": "d"},
						},
					},
				},
				DevEnvironments: chev2.CheClusterDevEnvironments{
					DefaultNamespace: chev2.DefaultNamespace{
						Template: "WorkspaceNamespaceName",
					},
					TrustedCerts: &chev2.TrustedCerts{
						GitTrustedCertsConfigMapName: "che-git-self-signed-cert",
					},
					Storage: chev2.WorkspaceStorage{
						Pvc: &chev2.PVC{
							ClaimSize:    "StorageClaimSize",
							StorageClass: "StorageClass",
						},
						PvcStrategy: "PvcStrategy",
					},
					DefaultPlugins: []chev2.WorkspaceDefaultPlugins{
						{
							Editor:  "Editor",
							Plugins: []string{"Plugins_1", "Plugins_2"},
						},
					},
					NodeSelector: map[string]string{"a": "b", "c": "d"},
					Tolerations: []corev1.Toleration{{
						Key:      "Key",
						Operator: "Operator",
						Value:    "Value",
						Effect:   "Effect",
					}},
				},
				ContainerRegistry: chev2.CheClusterContainerRegistry{
					Hostname:     "AirGapContainerRegistryHostname",
					Organization: "AirGapContainerRegistryOrganization",
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
				PostgresVersion:    "PostgresVersion",
			},
		}

		checlusterv1 := &chev1.CheCluster{}
		checlusterv2 := &chev2.CheCluster{}

		err := checlusterv1.ConvertFrom(checlusterv2Original)
		assert.Nil(t, err)

		err = checlusterv1.ConvertTo(checlusterv2)
		assert.Nil(t, err)

		assert.Equal(t, checlusterv2Original, checlusterv2)
	}
	onKubernetes(f)
	onOpenShift(f)
}

func TestRoundConvertCheClusterV1(t *testing.T) {
	f := func() {
		truststoreConfigMap := &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultServerTrustStoreConfigMapName,
				Namespace: "eclipse-che",
			},
		}

		k8sHelper := k8shelper.New()
		_, err := k8sHelper.GetClientset().CoreV1().ConfigMaps("eclipse-che").Create(context.TODO(), truststoreConfigMap, metav1.CreateOptions{})

		checlusterv1Orignal := &chev1.CheCluster{
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
					WorkspaceNamespaceDefault:           "WorkspaceNamespaceDefault",
					ServerTrustStoreConfigMapName:       constants.DefaultServerTrustStoreConfigMapName,
					GitSelfSignedCert:                   true,
					DashboardImage:                      "DashboardImage",
					DashboardImagePullPolicy:            "Always",
					DashboardMemoryLimit:                "200Mi",
					DashboardMemoryRequest:              "100Mi",
					DashboardCpuLimit:                   "2",
					DashboardCpuRequest:                 "1",
					DevfileRegistryImage:                "DevfileRegistryImage",
					DevfileRegistryPullPolicy:           "Always",
					DevfileRegistryMemoryLimit:          "200Mi",
					DevfileRegistryMemoryRequest:        "100Mi",
					DevfileRegistryCpuLimit:             "2",
					DevfileRegistryCpuRequest:           "1",
					ExternalDevfileRegistry:             true,
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
				},
				Database: chev1.CheClusterSpecDB{
					ExternalDb:              true,
					ChePostgresHostName:     "ChePostgresHostName",
					ChePostgresPort:         "ChePostgresPort",
					ChePostgresDb:           "ChePostgresDb",
					ChePostgresSecret:       "ChePostgresSecret",
					PostgresImage:           "PostgresImage",
					PostgresVersion:         "PostgresVersion",
					PostgresImagePullPolicy: "Always",
					PvcClaimSize:            "DatabasePvcClaimSize",
					ChePostgresContainerResources: chev1.ResourcesCustomSettings{
						Requests: chev1.Resources{
							Memory: "100Mi",
							Cpu:    "1",
						},
						Limits: chev1.Resources{
							Memory: "200Mi",
							Cpu:    "2",
						},
					},
				},
				Auth: chev1.CheClusterSpecAuth{
					IdentityProviderURL:               "IdentityProviderURL",
					OAuthClientName:                   "OAuthClientName",
					OAuthSecret:                       "OAuthSecret",
					OAuthScope:                        "OAuthScope",
					IdentityToken:                     "IdentityToken",
					GatewayAuthenticationSidecarImage: "GatewayAuthenticationSidecarImage",
					GatewayAuthorizationSidecarImage:  "GatewayAuthorizationSidecarImage",
				},
				Storage: chev1.CheClusterSpecStorage{
					PvcStrategy:                  "PvcStrategy",
					PvcClaimSize:                 "WorkspacePvcClaimSize",
					PostgresPVCStorageClassName:  "PostgresPVCStorageClassName",
					WorkspacePVCStorageClassName: "WorkspacePVCStorageClassName",
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
					Enable:          true,
					ControllerImage: "ControllerImage",
					RunningLimit:    "RunningLimit",
				},
				Dashboard: chev1.CheClusterSpecDashboard{
					Warning: "DashboardWarning",
				},
			},
			Status: chev1.CheClusterStatus{
				CheClusterRunning:                    "Available",
				CheVersion:                           "CheVersion",
				CheURL:                               "CheURL",
				Message:                              "Message",
				Reason:                               "Reason",
				DevfileRegistryURL:                   "DevfileRegistryURL",
				PluginRegistryURL:                    "PluginRegistryURL",
				GitServerTLSCertificateConfigMapName: "che-git-self-signed-cert",
				DevworkspaceStatus: chev1.LegacyDevworkspaceStatus{
					GatewayHost:         "CheURL",
					Message:             "Message",
					Reason:              "Reason",
					Phase:               chev1.ClusterPhaseActive,
					GatewayPhase:        chev1.GatewayPhaseEstablished,
					WorkspaceBaseDomain: "Domain",
				},
			},
		}

		checlusterv1 := &chev1.CheCluster{}
		checlusterv2 := &chev2.CheCluster{}

		err = checlusterv1Orignal.ConvertTo(checlusterv2)
		assert.Nil(t, err)

		err = checlusterv1.ConvertFrom(checlusterv2)
		assert.Nil(t, err)

		assert.Equal(t, checlusterv1Orignal, checlusterv1)
	}

	onKubernetes(f)
	onOpenShift(f)
}

func onKubernetes(f func()) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	f()
}

func onOpenShift(f func()) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	f()
}
