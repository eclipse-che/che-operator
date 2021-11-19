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
package gateway

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
)

func TestSyncAllToCluster(t *testing.T) {
	util.IsOpenShift = true
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ServerExposureStrategy: "single-host",
				},
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	err := SyncGatewayToCluster(deployContext)
	if err != nil {
		t.Fatalf("Failed to sync Gateway: %v", err)
	}

	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 2 {
		t.Fatalf("With classic multi-user, there should be 2 containers in the gateway, traefik and configbump. But it has '%d' containers.", len(deployment.Spec.Template.Spec.Containers))
	}

	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
}

func TestNativeUserGateway(t *testing.T) {
	util.IsOpenShift = true
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	nativeUserMode := true
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: orgv1.CheClusterSpec{
				Auth: orgv1.CheClusterSpecAuth{
					NativeUserMode: &nativeUserMode,
				},
				Server: orgv1.CheClusterSpecServer{
					ServerExposureStrategy: "single-host",
				},
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	err := SyncGatewayToCluster(deployContext)
	if err != nil {
		t.Fatalf("Failed to sync Gateway: %v", err)
	}

	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}

	if len(deployment.Spec.Template.Spec.Containers) != 4 {
		t.Fatalf("With native user mode, there should be 4 containers in the gateway.. But it has '%d' containers.", len(deployment.Spec.Template.Spec.Containers))
	}

	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "gateway" {
			if len(c.VolumeMounts) != 3 {
				t.Fatalf("gateway container should have 3 mounts, but it has '%d' ... \n%+v", len(c.VolumeMounts), c.VolumeMounts)
			}
		}
	}

	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}
}

func TestNoGatewayForMultiHost(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ServerExposureStrategy: "multi-host",
				},
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	err := SyncGatewayToCluster(deployContext)
	if err != nil {
		t.Fatalf("Failed to sync Gateway: %v", err)
	}

	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: GatewayServiceName, Namespace: "eclipse-che"}, deployment)
	if err == nil {
		t.Fatalf("Failed to get deployment: %v", err)
	} else {
		if v, ok := err.(errors.APIStatus); ok {
			if v.Status().Code != 404 {
				t.Fatalf("Deployment should not be found, thus code 404, but got '%d'", v.Status().Code)
			}
		} else {
			t.Fatalf("Wrong error returned.")
		}
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
		configmap := getGatewayOauthProxyConfigSpec(&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ExternalDevfileRegistry: true,
					ExternalPluginRegistry:  true,
				}}}, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		if strings.Contains(config, "skip_auth_regex") {
			t.Errorf("oauth config shold not contain any skip auth when both registries are external")
		}
	})

	t.Run("no devfile-registry auth", func(t *testing.T) {
		configmap := getGatewayOauthProxyConfigSpec(&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ExternalDevfileRegistry: false,
					ExternalPluginRegistry:  true,
				}}}, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		if !strings.Contains(config, "skip_auth_regex = \"^/devfile-registry\"") {
			t.Error("oauth config should skip auth for devfile registry", config)
		}
	})

	t.Run("skip plugin-registry auth", func(t *testing.T) {
		configmap := getGatewayOauthProxyConfigSpec(&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ExternalDevfileRegistry: true,
					ExternalPluginRegistry:  false,
				}}}, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		if !strings.Contains(config, "skip_auth_regex = \"^/plugin-registry\"") {
			t.Error("oauth config should skip auth for plugin registry", config)
		}
	})

	t.Run("skip both registries auth", func(t *testing.T) {
		configmap := getGatewayOauthProxyConfigSpec(&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ExternalDevfileRegistry: false,
					ExternalPluginRegistry:  false,
				}}}, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		if !strings.Contains(config, "skip_auth_regex = \"^/plugin-registry|^/devfile-registry\"") {
			t.Error("oauth config should skip auth for plugin and devfile registry.", config)
		}
	})

	t.Run("skip '/healthz' path", func(t *testing.T) {
		configmap := getGatewayOauthProxyConfigSpec(&orgv1.CheCluster{
			Spec: orgv1.CheClusterSpec{
				Auth: orgv1.CheClusterSpecAuth{
					NativeUserMode: util.NewBoolPointer(true),
				}}}, "blabol")
		config := configmap.Data["oauth-proxy.cfg"]
		assert.Contains(t, config, "/healthz$")
	})
}

func TestTokenValidityCheckOnOpenShiftNativeUser(t *testing.T) {
	onOpenShift4(func() {
		orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
		corev1.SchemeBuilder.AddToScheme(scheme.Scheme)

		cm, err := getGatewayServerConfigSpec(&deploy.DeployContext{
			CheCluster: &orgv1.CheCluster{
				Spec: orgv1.CheClusterSpec{
					Auth: orgv1.CheClusterSpecAuth{
						NativeUserMode: util.NewBoolPointer(true),
					},
				},
			},
			ClusterAPI: deploy.ClusterAPI{
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
	})
}

func onOpenShift4(f func()) {
	openshift := util.IsOpenShift
	openshiftv4 := util.IsOpenShift4

	util.IsOpenShift = true
	util.IsOpenShift4 = true

	f()

	util.IsOpenShift = openshift
	util.IsOpenShift4 = openshiftv4
}
