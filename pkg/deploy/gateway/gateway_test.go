//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package gateway

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"k8s.io/utils/pointer"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/yaml"
)

func TestSyncAllToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncGatewayToCluster(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	deployment := &appsv1.Deployment{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}

	assert.Lenf(t, deployment.Spec.Template.Spec.Containers, 4,
		"There should be 4 containers in the gateway. But it has '%d' containers.", len(deployment.Spec.Template.Spec.Containers))
	for _, c := range deployment.Spec.Template.Spec.Containers {
		assert.NotNil(t, c.Resources, "container '%s' has not set resources", c.Name)
	}

	service := &corev1.Service{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
}

func TestNativeUserGateway(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncGatewayToCluster(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	deployment := &appsv1.Deployment{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 4 {
		t.Fatalf("With native user mode, there should be 4 containers in the gateway.. But it has '%d' containers.", len(deployment.Spec.Template.Spec.Containers))
	}

	for _, c := range deployment.Spec.Template.Spec.Containers {
		assert.NotNil(t, c.Resources, "container '%s' has not set resources", c.Name)
		if c.Name == "gateway" {
			if len(c.VolumeMounts) != 3 {
				t.Fatalf("gateway container should have 3 mounts, but it has '%d' ... \n%+v", len(c.VolumeMounts), c.VolumeMounts)
			}
		}
	}

	service := &corev1.Service{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
}

func TestRandomCookieSecret(t *testing.T) {
	secret := generateRandomCookieSecret()
	if len(secret) != 24 {
		t.Fatalf("lenght of the secret should be 24")
	}

	_, err := base64.StdEncoding.Decode(make([]byte, 24), secret)
	if err != nil {
		t.Fatalf("Failed to decode the secret '%s'", err)
	}
}

func TestOauthProxyConfigUnauthorizedPaths(t *testing.T) {
	t.Run("no skip auth", func(t *testing.T) {
		ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					PluginRegistry: chev2.PluginRegistry{
						DisableInternalRegistry: true,
					},
				}},
		}).Build()

		configmap := getGatewayOauthProxyConfigSpec(ctx, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		if !strings.Contains(config, "skip_auth_regex = \"^/$|/healthz$|^/dashboard/static/preload|^/dashboard/assets/branding/loader.svg$\"") {
			t.Errorf("oauth config shold not contain any skip auth when both registries are external")
		}
	})

	t.Run("skip plugin registry", func(t *testing.T) {
		ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					PluginRegistry: chev2.PluginRegistry{
						DisableInternalRegistry: false,
						OpenVSXURL:              pointer.String(""),
					},
				}},
		}).Build()

		configmap := getGatewayOauthProxyConfigSpec(ctx, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		if !strings.Contains(config, "skip_auth_regex = \"^/plugin-registry|^/$|/healthz$|^/dashboard/static/preload|^/dashboard/assets/branding/loader.svg$\"") {
			t.Error("oauth config should skip auth for plugin and devfile registry.", config)
		}
	})

	t.Run("skip '/healthz' path", func(t *testing.T) {
		ctx := test.NewCtxBuilder().Build()
		configmap := getGatewayOauthProxyConfigSpec(ctx, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		assert.Contains(t, config, "/healthz$")
	})
}

func TestTokenValidityCheckOnOpenShiftNativeUser(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)

	cm, err := getGatewayServerConfigSpec(&chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{},
		ClusterAPI: chetypes.ClusterAPI{
			Scheme: scheme.Scheme,
		},
	})
	assert.NoError(t, err)

	cfg := &TraefikConfig{}

	assert.NoError(t, yaml.Unmarshal([]byte(cm.Data["server.yml"]), cfg))

	if assert.Contains(t, cfg.HTTP.Routers, "server") {
		assert.Contains(t, cfg.HTTP.Routers["server"].Middlewares, "server-token-check")
	}
	if assert.Contains(t, cfg.HTTP.Middlewares, "server-token-check") && assert.NotNil(t, cfg.HTTP.Middlewares["server-token-check"].ForwardAuth) {
		assert.Equal(t, "https://kubernetes.default.svc/apis/user.openshift.io/v1/users/~", cfg.HTTP.Middlewares["server-token-check"].ForwardAuth.Address)
	}
}

func TestCustomizeGatewayDeploymentAllImages(t *testing.T) {
	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:  constants.GatewayContainerName,
									Image: "gateway-image",
								},
								{
									Name:  constants.GatewayConfigSideCarContainerName,
									Image: "gateway-sidecar-image",
								},
								{
									Name:  constants.GatewayAuthenticationContainerName,
									Image: "gateway-authentication-image",
								},
								{
									Name:  constants.GatewayAuthorizationContainerName,
									Image: "gateway-authorization-image",
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := test.NewCtxBuilder().WithCheCluster(checluster).Build()

	deployment, err := getGatewayDeploymentSpec(ctx)
	assert.NoError(t, err)
	containers := deployment.Spec.Template.Spec.Containers
	assert.Equal(t, constants.GatewayContainerName, containers[0].Name)
	assert.Equal(t, "gateway-image", containers[0].Image)

	assert.Equal(t, constants.GatewayConfigSideCarContainerName, containers[1].Name)
	assert.Equal(t, "gateway-sidecar-image", containers[1].Image)

	assert.Equal(t, constants.GatewayAuthenticationContainerName, containers[2].Name)
	assert.Equal(t, "gateway-authentication-image", containers[2].Image)

	assert.Equal(t, constants.GatewayAuthorizationContainerName, containers[3].Name)
	assert.Equal(t, "gateway-authorization-image", containers[3].Image)
}

func TestCustomizeGatewayDeploymentSingleImage(t *testing.T) {
	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						Deployment: &chev2.Deployment{
							Containers: []chev2.Container{
								{
									Name:  constants.GatewayContainerName,
									Image: "gateway-image",
								},
							},
						},
					},
				},
			},
		},
	}
	ctx := test.NewCtxBuilder().WithCheCluster(checluster).Build()

	deployment, err := getGatewayDeploymentSpec(ctx)
	assert.NoError(t, err)

	containers := deployment.Spec.Template.Spec.Containers
	assert.Equal(t, constants.GatewayContainerName, containers[0].Name)
	assert.Equal(t, "gateway-image", containers[0].Image)

	assert.Equal(t, constants.GatewayConfigSideCarContainerName, containers[1].Name)
	assert.Equal(t, defaults.GetGatewayConfigSidecarImage(checluster), containers[1].Image)

	assert.Equal(t, constants.GatewayAuthenticationContainerName, containers[2].Name)
	assert.Equal(t, defaults.GetGatewayOpenShiftAuthenticationSidecarImage(checluster), containers[2].Image)

	assert.Equal(t, constants.GatewayAuthorizationContainerName, containers[3].Name)
	assert.Equal(t, defaults.GetGatewayOpenShiftAuthorizationSidecarImage(checluster), containers[3].Image)
}

func TestTraefikLogLevel(t *testing.T) {
	checluster := &chev2.CheCluster{
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						Traefik: &chev2.Traefik{
							LogLevel: "DEBUG",
						},
					},
				},
			},
		},
	}
	configmap := getGatewayTraefikConfigSpec(checluster)
	config := configmap.Data["traefik.yml"]
	if !strings.Contains(config, "level: \"DEBUG\"") {
		t.Error("log.level within traefik config should be \"DEBUG\"", config)
	}
}

func TestTraefikLogLevelDefault(t *testing.T) {
	configmap := getGatewayTraefikConfigSpec(&chev2.CheCluster{
		Spec: chev2.CheClusterSpec{},
	})
	config := configmap.Data["traefik.yml"]
	if !strings.Contains(config, "level: \"INFO\"") {
		t.Error("log.level within traefik config should be \"INFO\"", config)
	}
}

func TestKubeRbacProxyLogLevel(t *testing.T) {
	logLevel := int32(10)
	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					Gateway: chev2.Gateway{
						KubeRbacProxy: &chev2.KubeRbacProxy{
							LogLevel: &logLevel,
						},
					},
				},
			},
		},
	}
	ctx := test.NewCtxBuilder().WithCheCluster(checluster).Build()

	deployment, err := getGatewayDeploymentSpec(ctx)
	assert.NoError(t, err)

	containers := deployment.Spec.Template.Spec.Containers
	assert.Equal(t, constants.GatewayAuthorizationContainerName, containers[3].Name)
	assert.Equal(t, "--v=10", containers[3].Args[4])
}

func TestKubeRbacProxyLogLevelDefault(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	deployment, err := getGatewayDeploymentSpec(ctx)
	assert.NoError(t, err)

	containers := deployment.Spec.Template.Spec.Containers
	assert.Equal(t, constants.GatewayAuthorizationContainerName, containers[3].Name)
	assert.Equal(t, "--v=0", containers[3].Args[4])
}
