package gateway

import (
	"fmt"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getGatewayOauthProxyConfigSpec(instance *orgv1.CheCluster, cookieSecret string) corev1.ConfigMap {
	var config string
	if util.IsOpenShift {
		config = openshiftOauthProxyConfig(instance, cookieSecret)
	} else {
		config = kubernetesOauthProxyconfig(instance, cookieSecret)
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

func openshiftOauthProxyConfig(instance *orgv1.CheCluster, cookieSecret string) string {
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
		instance.Spec.Server.CheHost,
		instance.Spec.Auth.OAuthClientName,
		instance.Spec.Auth.OAuthSecret,
		GatewayServiceName,
		cookieSecret,
		skipAuthConfig(instance))
}

func kubernetesOauthProxyconfig(instance *orgv1.CheCluster, cookieSecret string) string {
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
pass_access_token = true
skip_provider_button = true
%s
`, GatewayServicePort,
		instance.Spec.Server.CheHost,
		instance.Spec.Auth.IdentityProviderURL,
		instance.Spec.Auth.OAuthClientName,
		instance.Spec.Auth.OAuthSecret,
		cookieSecret,
		skipAuthConfig(instance))
}

func skipAuthConfig(instance *orgv1.CheCluster) string {
	var skipAuthPaths []string
	if !instance.Spec.Server.ExternalPluginRegistry {
		skipAuthPaths = append(skipAuthPaths, "^/"+deploy.PluginRegistryName)
	}
	if !instance.Spec.Server.ExternalDevfileRegistry {
		skipAuthPaths = append(skipAuthPaths, "^/"+deploy.DevfileRegistryName)
	}
	if util.IsNativeUserModeEnabled(instance) {
		skipAuthPaths = append(skipAuthPaths, "/healthz$")
	}
	if len(skipAuthPaths) > 0 {
		propName := "skip_auth_routes"
		if util.IsOpenShift {
			propName = "skip_auth_regex"
		}
		return fmt.Sprintf("%s = \"%s\"", propName, strings.Join(skipAuthPaths, "|"))
	}
	return ""
}
