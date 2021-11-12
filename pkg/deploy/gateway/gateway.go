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
	"fmt"
	"io/ioutil"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/sirupsen/logrus"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// GatewayServiceName is the name of the service which through which the gateway can be accessed
	GatewayServiceName = "che-gateway"
	GatewayServicePort = 8080

	serverComponentName        = "server"
	gatewayKubeAuthConfigName  = "che-gateway-route-kube-auth"
	gatewayConfigComponentName = "che-gateway-config"
	gatewayOauthSecretName     = "che-gateway-oauth-secret"
	GatewayConfigMapNamePrefix = "che-gateway-route-"
)

var (
	serviceAccountDiffOpts = cmpopts.IgnoreFields(corev1.ServiceAccount{}, "TypeMeta", "ObjectMeta", "Secrets", "ImagePullSecrets")
	roleDiffOpts           = cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta", "ObjectMeta")
	roleBindingDiffOpts    = cmpopts.IgnoreFields(rbac.RoleBinding{}, "TypeMeta", "ObjectMeta")
	configMapDiffOpts      = cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta")
	secretDiffOpts         = cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta")
)

// SyncGatewayToCluster installs or deletes the gateway based on the custom resource configuration
func SyncGatewayToCluster(deployContext *deploy.DeployContext) error {
	if (util.GetServerExposureStrategy(deployContext.CheCluster) == "single-host" &&
		deploy.GetSingleHostExposureType(deployContext.CheCluster) == deploy.GatewaySingleHostExposureType) ||
		deployContext.CheCluster.Spec.DevWorkspace.Enable {
		return syncAll(deployContext)
	}

	return deleteAll(deployContext)
}

func syncAll(deployContext *deploy.DeployContext) error {
	instance := deployContext.CheCluster
	sa := getGatewayServiceAccountSpec(instance)
	if _, err := deploy.Sync(deployContext, &sa, serviceAccountDiffOpts); err != nil {
		return err
	}

	role := getGatewayRoleSpec(instance)
	if _, err := deploy.Sync(deployContext, &role, roleDiffOpts); err != nil {
		return err
	}

	roleBinding := getGatewayRoleBindingSpec(instance)
	if _, err := deploy.Sync(deployContext, &roleBinding, roleBindingDiffOpts); err != nil {
		return err
	}

	if util.IsOpenShift && deployContext.CheCluster.IsNativeUserModeEnabled() {
		if oauthSecret, err := getGatewaySecretSpec(deployContext); err == nil {
			if _, err := deploy.Sync(deployContext, oauthSecret, secretDiffOpts); err != nil {
				return err
			}
			oauthProxyConfig := getGatewayOauthProxyConfigSpec(instance, string(oauthSecret.Data["cookie_secret"]))
			if _, err := deploy.Sync(deployContext, &oauthProxyConfig, configMapDiffOpts); err != nil {
				return err
			}
		} else {
			return err
		}

		if headerRewritePluginConfig, err := getGatewayHeaderRewritePluginConfigSpec(instance); err == nil {
			if _, err := deploy.Sync(deployContext, headerRewritePluginConfig, configMapDiffOpts); err != nil {
				return err
			}
		} else {
			return err
		}

		kubeRbacProxyConfig := getGatewayKubeRbacProxyConfigSpec(instance)
		if _, err := deploy.Sync(deployContext, &kubeRbacProxyConfig, configMapDiffOpts); err != nil {
			return err
		}
	}

	traefikConfig := getGatewayTraefikConfigSpec(instance)
	if _, err := deploy.Sync(deployContext, &traefikConfig, configMapDiffOpts); err != nil {
		return err
	}

	depl := getGatewayDeploymentSpec(instance)
	if _, err := deploy.Sync(deployContext, &depl, deploy.DefaultDeploymentDiffOpts); err != nil {
		return err
	}

	service := getGatewayServiceSpec(instance)
	if _, err := deploy.Sync(deployContext, &service, deploy.ServiceDefaultDiffOpts); err != nil {
		return err
	}

	if serverConfig, cfgErr := getGatewayServerConfigSpec(deployContext); cfgErr == nil {
		if _, err := deploy.Sync(deployContext, &serverConfig, configMapDiffOpts); err != nil {
			return err
		}
	}

	return nil
}

func getGatewaySecretSpec(deployContext *deploy.DeployContext) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	exists, err := deploy.GetNamespacedObject(deployContext, gatewayOauthSecretName, secret)
	if err == nil && exists {
		if _, ok := secret.Data["cookie_secret"]; !ok {
			logrus.Info("che-gateway-secret found, but does not contain `cookie_secret` value. Regenerating...")
			return generateOauthSecretSpec(deployContext), nil
		}
		return secret, nil
	} else if err == nil && !exists {
		return generateOauthSecretSpec(deployContext), nil
	} else {
		return nil, err
	}
}

func generateOauthSecretSpec(deployContext *deploy.DeployContext) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayOauthSecretName,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    deploy.GetLabels(deployContext.CheCluster, GatewayServiceName),
		},
		Data: map[string][]byte{
			"cookie_secret": generateRandomCookieSecret(),
		},
	}
}

func deleteAll(deployContext *deploy.DeployContext) error {
	instance := deployContext.CheCluster
	clusterAPI := deployContext.ClusterAPI

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &deployment); err != nil {
		return err
	}

	serverConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayConfigMapNamePrefix + serverComponentName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &serverConfig); err != nil {
		return err
	}

	traefikConfig := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config",
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &traefikConfig); err == nil {
		return err
	}

	roleBinding := rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &roleBinding); err == nil {
		return err
	}

	role := rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &role); err == nil {
		return err
	}

	sa := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
		},
	}
	if err := delete(clusterAPI, &sa); err == nil {
		return err
	}

	return nil
}

func delete(clusterAPI deploy.ClusterAPI, obj metav1.Object) error {
	key := client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	ro := obj.(client.Object)
	if getErr := clusterAPI.Client.Get(context.TODO(), key, ro); getErr == nil {
		if err := clusterAPI.Client.Delete(context.TODO(), ro); err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
		}
	}

	return nil
}

func DeleteGatewayRouteConfig(componentName string, deployContext *deploy.DeployContext) error {
	obj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayConfigMapNamePrefix + componentName,
			Namespace: deployContext.CheCluster.Namespace,
		},
	}

	return delete(deployContext.ClusterAPI, obj)
}

// below functions declare the desired states of the various objects required for the gateway

func getGatewayServerConfigSpec(deployContext *deploy.DeployContext) (corev1.ConfigMap, error) {
	cfg := CreateCommonTraefikConfig(
		serverComponentName,
		"Path(`/`, `/f`) || PathPrefix(`/api`, `/swagger`, `/_app`)",
		1,
		"http://"+deploy.CheServiceName+":8080",
		[]string{})

	if util.IsOpenShift && deployContext.CheCluster.IsNativeUserModeEnabled() {
		cfg.AddAuthHeaderRewrite(serverComponentName)
		// native user mode is currently only available on OpenShift but let's be defensive here so that
		// this doesn't break once we enable it on Kubernetes, too. Token check will have to work
		// differently on Kuberentes.
		if util.IsOpenShift {
			cfg.AddOpenShiftTokenCheck(serverComponentName)
		}
	}

	return GetConfigmapForGatewayConfig(deployContext, serverComponentName, cfg)
}

func GetConfigmapForGatewayConfig(
	deployContext *deploy.DeployContext,
	componentName string,
	gatewayConfig *TraefikConfig) (corev1.ConfigMap, error) {

	gatewayConfigContent, err := yaml.Marshal(gatewayConfig)
	if err != nil {
		logrus.Error(err, "can't serialize traefik config")
	}

	ret := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayConfigMapNamePrefix + componentName,
			Namespace: deployContext.CheCluster.Namespace,
			Labels: util.MergeMaps(
				deploy.GetLabels(deployContext.CheCluster, gatewayConfigComponentName),
				util.GetMapValue(deployContext.CheCluster.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels)),
		},
		Data: map[string]string{
			componentName + ".yml": string(gatewayConfigContent),
		},
	}

	controllerutil.SetControllerReference(deployContext.CheCluster, &ret, deployContext.ClusterAPI.Scheme)

	return ret, nil
}

func getGatewayServiceAccountSpec(instance *orgv1.CheCluster) corev1.ServiceAccount {
	return corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
	}
}

func getGatewayRoleSpec(instance *orgv1.CheCluster) rbac.Role {
	return rbac.Role{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "Role",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Rules: []rbac.PolicyRule{
			{
				Verbs:     []string{"watch", "get", "list"},
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
			},
		},
	}
}

func getGatewayRoleBindingSpec(instance *orgv1.CheCluster) rbac.RoleBinding {
	return rbac.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbac.SchemeGroupVersion.String(),
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		RoleRef: rbac.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     GatewayServiceName,
		},
		Subjects: []rbac.Subject{
			{
				Kind: "ServiceAccount",
				Name: GatewayServiceName,
			},
		},
	}
}

func getGatewayOauthProxyConfigSpec(instance *orgv1.CheCluster, cookieSecret string) corev1.ConfigMap {
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
			"oauth-proxy.cfg": fmt.Sprintf(`
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
				skipAuthConfig(instance)),
		},
	}
}

func skipAuthConfig(instance *orgv1.CheCluster) string {
	var skipAuthPaths []string
	if !instance.Spec.Server.ExternalPluginRegistry {
		skipAuthPaths = append(skipAuthPaths, "^/"+deploy.PluginRegistryName)
	}
	if !instance.Spec.Server.ExternalDevfileRegistry {
		skipAuthPaths = append(skipAuthPaths, "^/"+deploy.DevfileRegistryName)
	}
	if util.IsOpenShift && instance.IsNativeUserModeEnabled() {
		skipAuthPaths = append(skipAuthPaths, "/healthz$")
	}
	if len(skipAuthPaths) > 0 {
		return fmt.Sprintf("skip_auth_regex = \"%s\"", strings.Join(skipAuthPaths, "|"))
	}
	return ""
}

func getGatewayKubeRbacProxyConfigSpec(instance *orgv1.CheCluster) corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config-kube-rbac-proxy",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Data: map[string]string{
			"authorization-config.yaml": `
authorization:
  rewrites:
    byQueryParameter:
      name: "namespace"
  resourceAttributes:
    apiVersion: v1
    resource: services
    namespace: "{{ .Value }}"`,
		},
	}
}

func generateRandomCookieSecret() []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(util.GeneratePasswd(16))))
}

func getGatewayHeaderRewritePluginConfigSpec(instance *orgv1.CheCluster) (*corev1.ConfigMap, error) {
	headerRewrite, err := ioutil.ReadFile("/tmp/header-rewrite-traefik-plugin/headerRewrite.go")
	if err != nil {
		if !util.IsTestMode() {
			return nil, err
		}
	}
	pluginMeta, err := ioutil.ReadFile("/tmp/header-rewrite-traefik-plugin/.traefik.yml")
	if err != nil {
		if !util.IsTestMode() {
			return nil, err
		}
	}

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config-header-rewrite-traefik-plugin",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Data: map[string]string{
			"headerRewrite.go": string(headerRewrite),
			".traefik.yml":     string(pluginMeta),
		},
	}, nil
}

func getGatewayTraefikConfigSpec(instance *orgv1.CheCluster) corev1.ConfigMap {
	traefikPort := GatewayServicePort
	if util.IsOpenShift && instance.IsNativeUserModeEnabled() {
		traefikPort = 8081
	}
	data := fmt.Sprintf(`
entrypoints:
  http:
    address: ":%d"
    forwardedHeaders:
      insecure: true
  sink:
    address: ":8090"
ping:
  entryPoint: "sink"
global:
  checkNewVersion: false
  sendAnonymousUsage: false
providers:
  file:
    directory: "/dynamic-config"
    watch: true
log:
  level: "INFO"`, traefikPort)

	if util.IsOpenShift && instance.IsNativeUserModeEnabled() {
		data += `
experimental:
  localPlugins:
    header-rewrite:
      moduleName: github.com/che-incubator/header-rewrite-traefik-plugin`
	}

	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Data: map[string]string{
			"traefik.yml": data,
		},
	}
}

func getGatewayDeploymentSpec(instance *orgv1.CheCluster) appsv1.Deployment {
	terminationGracePeriodSeconds := int64(10)

	deployLabels, labelsSelector := deploy.GetLabelsAndSelector(instance, GatewayServiceName)

	return appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deployLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labelsSelector,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: deployLabels,
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					ServiceAccountName:            GatewayServiceName,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					Containers:                    getContainersSpec(instance),
					Volumes:                       getVolumesSpec(instance),
				},
			},
		},
	}
}

func getContainersSpec(instance *orgv1.CheCluster) []corev1.Container {
	configLabelsMap := util.GetMapValue(instance.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels)
	gatewayImage := util.GetValue(instance.Spec.Server.SingleHostGatewayImage, deploy.DefaultSingleHostGatewayImage(instance))
	configSidecarImage := util.GetValue(instance.Spec.Server.SingleHostGatewayConfigSidecarImage, deploy.DefaultSingleHostGatewayConfigSidecarImage(instance))
	authnImage := util.GetValue(instance.Spec.Auth.GatewayAuthenticationSidecarImage, deploy.DefaultGatewayAuthenticationSidecarImage(instance))
	authzImage := util.GetValue(instance.Spec.Auth.GatewayAuthorizationSidecarImage, deploy.DefaultGatewayAuthorizationSidecarImage(instance))
	configLabels := labels.FormatLabels(configLabelsMap)

	containers := []corev1.Container{
		{
			Name:            "gateway",
			Image:           gatewayImage,
			ImagePullPolicy: corev1.PullAlways,
			VolumeMounts:    getTraefikContainerVolumeMounts(instance),
		},
		{
			Name:            "configbump",
			Image:           configSidecarImage,
			ImagePullPolicy: corev1.PullAlways,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "dynamic-config",
					MountPath: "/dynamic-config",
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "CONFIG_BUMP_DIR",
					Value: "/dynamic-config",
				},
				{
					Name:  "CONFIG_BUMP_LABELS",
					Value: configLabels,
				},
				{
					Name: "CONFIG_BUMP_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				},
			},
		},
	}

	if util.IsOpenShift && instance.IsNativeUserModeEnabled() {
		containers = append(containers,
			corev1.Container{
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
				Ports: []corev1.ContainerPort{
					{ContainerPort: GatewayServicePort, Protocol: "TCP"},
				},
			},
			corev1.Container{
				Name:            "kube-rbac-proxy",
				Image:           authzImage,
				ImagePullPolicy: corev1.PullAlways,
				Args: []string{
					"--insecure-listen-address=0.0.0.0:8089",
					"--upstream=http://127.0.0.1:8090/ping",
					"--logtostderr=true",
					"--config-file=/etc/kube-rbac-proxy/authorization-config.yaml",
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "kube-rbac-proxy-config",
						MountPath: "/etc/kube-rbac-proxy",
					},
				},
			})
	}

	return containers
}

func getTraefikContainerVolumeMounts(instance *orgv1.CheCluster) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "static-config",
			MountPath: "/etc/traefik",
		},
		{
			Name:      "dynamic-config",
			MountPath: "/dynamic-config",
		},
	}
	if util.IsOpenShift && instance.IsNativeUserModeEnabled() {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "header-rewrite-traefik-plugin",
			MountPath: "/plugins-local/src/github.com/che-incubator/header-rewrite-traefik-plugin",
		})
	}

	return mounts
}

func getVolumesSpec(instance *orgv1.CheCluster) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "static-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "che-gateway-config",
					},
				},
			},
		},
		{
			Name: "dynamic-config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	if util.IsOpenShift && instance.IsNativeUserModeEnabled() {
		volumes = append(volumes, corev1.Volume{
			Name: "oauth-proxy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "che-gateway-config-oauth-proxy",
					},
				},
			},
		})

		volumes = append(volumes, corev1.Volume{
			Name: "header-rewrite-traefik-plugin",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "che-gateway-config-header-rewrite-traefik-plugin",
					},
				},
			},
		})

		volumes = append(volumes, corev1.Volume{
			Name: "kube-rbac-proxy-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "che-gateway-config-kube-rbac-proxy",
					},
				},
			},
		})
	}

	return volumes
}

func getGatewayServiceSpec(instance *orgv1.CheCluster) corev1.Service {
	return corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      GatewayServiceName,
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
		},
		Spec: corev1.ServiceSpec{
			Selector:        deploy.GetLabels(instance, GatewayServiceName),
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "gateway-http",
					Port:       GatewayServicePort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(GatewayServicePort),
				},
				{
					Name:       "gateway-kube-authz",
					Port:       8089,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(8089),
				},
			},
		},
	}
}
