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

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
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
	memoryRequest := resource.MustParse("148Mi")
	cpuRequest := resource.MustParse("1")
	memoryLimit := resource.MustParse("228Mi")
	cpuLimit := resource.MustParse("2")

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
					PluginRegistry: chev2.PluginRegistry{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:            "plugin-registry",
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
											Memory: &memoryRequest,
											Cpu:    &cpuRequest,
										},
										Limits: &chev2.ResourceList{
											Memory: &memoryLimit,
											Cpu:    &cpuLimit,
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
											Memory: &memoryRequest,
											Cpu:    &cpuRequest,
										},
										Limits: &chev2.ResourceList{
											Memory: &memoryLimit,
											Cpu:    &cpuLimit,
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
											Memory: &memoryRequest,
											Cpu:    &cpuRequest,
										},
										Limits: &chev2.ResourceList{
											Memory: &memoryLimit,
											Cpu:    &cpuLimit,
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
						PerUserStrategyPvcConfig: &chev2.PVC{
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
					MaxNumberOfRunningWorkspacesPerUser: pointer.Int64Ptr(10),
					User: &chev2.UserConfiguration{
						ClusterRoles: []string{
							"ClusterRoles_1",
							"ClusterRoles_2",
						},
					},
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

func onKubernetes(f func()) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	f()
}

func onOpenShift(f func()) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	f()
}
