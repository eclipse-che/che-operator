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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getGatewayOauthProxyConfigSpec(ctx *deploy.DeployContext, cookieSecret string) corev1.ConfigMap {
	instance := ctx.CheCluster

	var config string
	if util.IsOpenShift {
		config = openshiftOauthProxyConfig(ctx, cookieSecret)
	} else {
		config = kubernetesOauthProxyconfig(ctx, cookieSecret)
	}
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config-oauth-proxy",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Data: map[string]string{
			"oauth-proxy.cfg": config,
		},
	}
}

func openshiftOauthProxyConfig(ctx *deploy.DeployContext, cookieSecret string) string {
	return fmt.Sprintf(`
http_address = ":%d"
https_address = ""
provider = "openshift"
redirect_url = "https://%s/oauth/callback"
upstreams = [
	"http://127.0.0.1:8081/"
]
client_id = "%s"
client_secret = "%s"
scope = "user:full"
openshift_service_account = "%s"
cookie_secret = "%s"
cookie_expire = "24h0m0s"
email_domains = "*"
cookie_httponly = false
pass_access_token = true
skip_provider_button = true
%s
`, GatewayServicePort,
		ctx.CheCluster.GetCheHost(),
		ctx.CheCluster.Spec.Auth.OAuthClientName,
		ctx.CheCluster.Spec.Auth.OAuthSecret,
		GatewayServiceName,
		cookieSecret,
		skipAuthConfig(ctx.CheCluster))
}

func kubernetesOauthProxyconfig(ctx *deploy.DeployContext, cookieSecret string) string {
	return fmt.Sprintf(`
proxy_prefix = "/oauth"
http_address = ":%d"
https_address = ""
provider = "oidc"
redirect_url = "https://%s/oauth/callback"
oidc_issuer_url = "%s"
insecure_oidc_skip_issuer_verification = true
ssl_insecure_skip_verify = true
upstreams = [
	"http://127.0.0.1:8081/"
]
client_id = "%s"
client_secret = "%s"
cookie_secret = "%s"
cookie_expire = "24h0m0s"
email_domains = "*"
cookie_httponly = false
pass_authorization_header = true
skip_provider_button = true
%s
`, GatewayServicePort,
		ctx.CheCluster.GetCheHost(),
		ctx.CheCluster.Spec.Auth.IdentityProviderURL,
		ctx.CheCluster.Spec.Auth.OAuthClientName,
		ctx.CheCluster.Spec.Auth.OAuthSecret,
		cookieSecret,
		skipAuthConfig(ctx.CheCluster))
}

func skipAuthConfig(instance *orgv1.CheCluster) string {
	var skipAuthPaths []string
	if !instance.Spec.Server.ExternalPluginRegistry {
		skipAuthPaths = append(skipAuthPaths, "^/"+deploy.PluginRegistryName)
	}
	if !instance.Spec.Server.ExternalDevfileRegistry {
		skipAuthPaths = append(skipAuthPaths, "^/"+deploy.DevfileRegistryName)
	}
	skipAuthPaths = append(skipAuthPaths, "^/$")
	skipAuthPaths = append(skipAuthPaths, "/healthz$")
	skipAuthPaths = append(skipAuthPaths, "^/dashboard/static/preload")
	if len(skipAuthPaths) > 0 {
		propName := "skip_auth_routes"
		if util.IsOpenShift {
			propName = "skip_auth_regex"
		}
		return fmt.Sprintf("%s = \"%s\"", propName, strings.Join(skipAuthPaths, "|"))
	}
	return ""
}

func getOauthProxyContainerSpec(ctx *deploy.DeployContext) corev1.Container {
	authnImage := util.GetValue(ctx.CheCluster.Spec.Auth.GatewayAuthenticationSidecarImage, deploy.DefaultGatewayAuthenticationSidecarImage(ctx.CheCluster))
	return corev1.Container{
		Name:            "oauth-proxy",
		Image:           authnImage,
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"--config=/etc/oauth-proxy/oauth-proxy.cfg",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "oauth-proxy-config",
				MountPath: "/etc/oauth-proxy",
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.5"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.1"),
			},
		},
		Ports: []corev1.ContainerPort{
			{ContainerPort: GatewayServicePort, Protocol: "TCP"},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "http_proxy",
				Value: ctx.Proxy.HttpProxy,
			},
			{
				Name:  "https_proxy",
				Value: ctx.Proxy.HttpsProxy,
			},
			{
				Name:  "no_proxy",
				Value: ctx.Proxy.NoProxy,
			},
		},
	}
}

func getOauthProxyConfigVolume() corev1.Volume {
	return corev1.Volume{
		Name: "oauth-proxy-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "che-gateway-config-oauth-proxy",
				},
			},
		},
	}
}
