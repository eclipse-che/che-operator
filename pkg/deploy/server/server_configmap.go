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

package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	deploytls "github.com/eclipse-che/che-operator/pkg/deploy/tls"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	CheConfigMapName = "che"
)

type CheConfigMap struct {
	CheHost                              string `json:"CHE_HOST"`
	CheMultiUser                         string `json:"CHE_MULTIUSER"`
	ChePort                              string `json:"CHE_PORT"`
	CheApi                               string `json:"CHE_API"`
	CheApiInternal                       string `json:"CHE_API_INTERNAL"`
	CheWebSocketEndpoint                 string `json:"CHE_WEBSOCKET_ENDPOINT"`
	CheWebSocketInternalEndpoint         string `json:"CHE_WEBSOCKET_INTERNAL_ENDPOINT"`
	CheDebugServer                       string `json:"CHE_DEBUG_SERVER"`
	CheMetricsEnabled                    string `json:"CHE_METRICS_ENABLED"`
	CheInfrastructureActive              string `json:"CHE_INFRASTRUCTURE_ACTIVE"`
	CheInfraKubernetesServiceAccountName string `json:"CHE_INFRA_KUBERNETES_SERVICE__ACCOUNT__NAME"`
	CheInfraKubernetesUserClusterRoles   string `json:"CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"`
	DefaultTargetNamespace               string `json:"CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"`
	NamespaceCreationAllowed             string `json:"CHE_INFRA_KUBERNETES_NAMESPACE_CREATION__ALLOWED"`
	PvcStrategy                          string `json:"CHE_INFRA_KUBERNETES_PVC_STRATEGY"`
	PvcClaimSize                         string `json:"CHE_INFRA_KUBERNETES_PVC_QUANTITY"`
	WorkspacePvcStorageClassName         string `json:"CHE_INFRA_KUBERNETES_PVC_STORAGE__CLASS__NAME"`
	TlsSupport                           string `json:"CHE_INFRA_OPENSHIFT_TLS__ENABLED"`
	K8STrustCerts                        string `json:"CHE_INFRA_KUBERNETES_TRUST__CERTS"`
	CheLogLevel                          string `json:"CHE_LOG_LEVEL"`
	IdentityProviderUrl                  string `json:"CHE_OIDC_AUTH__SERVER__URL,omitempty"`
	IdentityProviderInternalURL          string `json:"CHE_OIDC_AUTH__INTERNAL__SERVER__URL,omitempty"`
	OpenShiftIdentityProvider            string `json:"CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"`
	JavaOpts                             string `json:"JAVA_OPTS"`
	PluginRegistryUrl                    string `json:"CHE_WORKSPACE_PLUGIN__REGISTRY__URL,omitempty"`
	PluginRegistryInternalUrl            string `json:"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL,omitempty"`
	DevfileRegistryUrl                   string `json:"CHE_WORKSPACE_DEVFILE__REGISTRY__URL,omitempty"`
	DevfileRegistryInternalUrl           string `json:"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL,omitempty"`
	CheJGroupsKubernetesLabels           string `json:"KUBERNETES_LABELS,omitempty"`
	CheTrustedCABundlesConfigMap         string `json:"CHE_TRUSTED__CA__BUNDLES__CONFIGMAP,omitempty"`
	ServerStrategy                       string `json:"CHE_INFRA_KUBERNETES_SERVER__STRATEGY"`
	WorkspaceExposure                    string `json:"CHE_INFRA_KUBERNETES_SINGLEHOST_WORKSPACE_EXPOSURE"`
	SingleHostGatewayConfigMapLabels     string `json:"CHE_INFRA_KUBERNETES_SINGLEHOST_GATEWAY_CONFIGMAP__LABELS"`
	CheDevWorkspacesEnabled              string `json:"CHE_DEVWORKSPACES_ENABLED"`
	Http2Disable                         string `json:"HTTP2_DISABLE"`
}

// GetCheConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func (s *CheServerReconciler) getCheConfigMapData(ctx *chetypes.DeployContext) (cheEnv map[string]string, err error) {
	identityProviderURL := ctx.CheCluster.Spec.Networking.Auth.IdentityProviderURL

	infra := "kubernetes"
	openShiftIdentityProviderId := "NULL"
	if infrastructure.IsOpenShift() {
		infra = "openshift"
		openShiftIdentityProviderId = "openshift-v4"
	}

	proxyJavaOpts := ""
	cheWorkspaceNoProxy := ctx.Proxy.NoProxy
	if ctx.Proxy.HttpProxy != "" {
		if ctx.Proxy.NoProxy == "" {
			cheWorkspaceNoProxy = os.Getenv("KUBERNETES_SERVICE_HOST")
		} else {
			cheWorkspaceNoProxy = cheWorkspaceNoProxy + "," + os.Getenv("KUBERNETES_SERVICE_HOST")
		}
		proxyJavaOpts, err = deploy.GenerateProxyJavaOpts(ctx.Proxy, cheWorkspaceNoProxy)
		if err != nil {
			logrus.Errorf("Failed to generate java proxy options: %v", err)
		}
	}

	ingressDomain := ctx.CheCluster.Spec.Networking.Domain
	tlsSecretName := ctx.CheCluster.Spec.Networking.TlsSecretName

	securityContextFsGroup := strconv.FormatInt(constants.DefaultSecurityContextFsGroup, 10)
	securityContextRunAsUser := strconv.FormatInt(constants.DefaultSecurityContextRunAsUser, 10)
	if ctx.CheCluster.Spec.Components.CheServer.Deployment != nil {
		if ctx.CheCluster.Spec.Components.CheServer.Deployment.SecurityContext != nil {
			if ctx.CheCluster.Spec.Components.CheServer.Deployment.SecurityContext.FsGroup != nil {
				securityContextFsGroup = strconv.FormatInt(*ctx.CheCluster.Spec.Components.CheServer.Deployment.SecurityContext.FsGroup, 10)
			}
			if ctx.CheCluster.Spec.Components.CheServer.Deployment.SecurityContext.RunAsUser != nil {
				securityContextRunAsUser = strconv.FormatInt(*ctx.CheCluster.Spec.Components.CheServer.Deployment.SecurityContext.RunAsUser, 10)
			}
		}
	}

	ingressClass := utils.GetValue(ctx.CheCluster.Spec.Networking.Annotations["kubernetes.io/ingress.class"], constants.DefaultIngressClass)

	// grab first the devfile registry url which is deployed by operator
	devfileRegistryURL := ctx.CheCluster.Status.DevfileRegistryURL

	for _, r := range ctx.CheCluster.Spec.Components.DevfileRegistry.ExternalDevfileRegistries {
		if strings.Index(devfileRegistryURL, r.Url) == -1 {
			devfileRegistryURL += " " + r.Url
		}
	}
	devfileRegistryURL = strings.TrimSpace(devfileRegistryURL)

	pluginRegistryURL := ctx.CheCluster.Status.PluginRegistryURL
	for _, r := range ctx.CheCluster.Spec.Components.PluginRegistry.ExternalPluginRegistries {
		if strings.Index(pluginRegistryURL, r.Url) == -1 {
			pluginRegistryURL += " " + r.Url
		}
	}
	pluginRegistryURL = strings.TrimSpace(pluginRegistryURL)

	cheLogLevel := utils.GetValue(ctx.CheCluster.Spec.Components.CheServer.LogLevel, constants.DefaultServerLogLevel)
	cheDebug := "false"
	if ctx.CheCluster.Spec.Components.CheServer.Debug != nil {
		cheDebug = strconv.FormatBool(*ctx.CheCluster.Spec.Components.CheServer.Debug)
	}
	cheMetrics := strconv.FormatBool(ctx.CheCluster.Spec.Components.Metrics.Enable)
	cheLabels := labels.FormatLabels(deploy.GetLabels(defaults.GetCheFlavor()))

	singleHostGatewayConfigMapLabels := ""
	if len(ctx.CheCluster.Spec.Networking.Auth.Gateway.ConfigLabels) != 0 {
		singleHostGatewayConfigMapLabels = labels.FormatLabels(ctx.CheCluster.Spec.Networking.Auth.Gateway.ConfigLabels)
	} else {
		singleHostGatewayConfigMapLabels = labels.FormatLabels(constants.DefaultSingleHostGatewayConfigMapLabels)

	}
	workspaceNamespaceDefault := ctx.CheCluster.GetDefaultNamespace()
	namespaceCreationAllowed := strconv.FormatBool(constants.DefaultAutoProvision)
	if ctx.CheCluster.Spec.DevEnvironments.DefaultNamespace.AutoProvision != nil {
		namespaceCreationAllowed = strconv.FormatBool(*ctx.CheCluster.Spec.DevEnvironments.DefaultNamespace.AutoProvision)
	}

	cheAPI := "https://" + ctx.CheHost + "/api"
	var pluginRegistryInternalURL, devfileRegistryInternalURL string

	// If there is a devfile registry deployed by operator
	if !ctx.CheCluster.Spec.Components.DevfileRegistry.DisableInternalRegistry {
		devfileRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080", constants.DevfileRegistryName, ctx.CheCluster.Namespace)
	}

	if !ctx.CheCluster.Spec.Components.PluginRegistry.DisableInternalRegistry {
		pluginRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080/v3", constants.PluginRegistryName, ctx.CheCluster.Namespace)
	}

	cheInternalAPI := fmt.Sprintf("http://%s.%s.svc:8080/api", deploy.CheServiceName, ctx.CheCluster.Namespace)
	webSocketInternalEndpoint := fmt.Sprintf("ws://%s.%s.svc:8080/api/websocket", deploy.CheServiceName, ctx.CheCluster.Namespace)
	webSocketEndpoint := "wss://" + ctx.CheHost + "/api/websocket"
	cheWorkspaceServiceAccount := "NULL"

	data := &CheConfigMap{
		CheMultiUser:                         "true",
		CheHost:                              ctx.CheHost,
		ChePort:                              "8080",
		CheApi:                               cheAPI,
		CheApiInternal:                       cheInternalAPI,
		CheWebSocketEndpoint:                 webSocketEndpoint,
		CheWebSocketInternalEndpoint:         webSocketInternalEndpoint,
		CheDebugServer:                       cheDebug,
		CheInfrastructureActive:              infra,
		CheInfraKubernetesServiceAccountName: cheWorkspaceServiceAccount,
		DefaultTargetNamespace:               workspaceNamespaceDefault,
		NamespaceCreationAllowed:             namespaceCreationAllowed,
		TlsSupport:                           "true",
		K8STrustCerts:                        "true",
		CheLogLevel:                          cheLogLevel,
		OpenShiftIdentityProvider:            openShiftIdentityProviderId,
		JavaOpts:                             constants.DefaultJavaOpts + " " + proxyJavaOpts,
		PluginRegistryUrl:                    pluginRegistryURL,
		PluginRegistryInternalUrl:            pluginRegistryInternalURL,
		DevfileRegistryUrl:                   devfileRegistryURL,
		DevfileRegistryInternalUrl:           devfileRegistryInternalURL,
		CheJGroupsKubernetesLabels:           cheLabels,
		CheMetricsEnabled:                    cheMetrics,
		CheTrustedCABundlesConfigMap:         deploytls.CheAllCACertsConfigMapName,
		ServerStrategy:                       "single-host",
		WorkspaceExposure:                    "gateway",
		SingleHostGatewayConfigMapLabels:     singleHostGatewayConfigMapLabels,
		CheDevWorkspacesEnabled:              strconv.FormatBool(true),
		// Disable HTTP2 protocol.
		// Fix issue with creating config maps on the cluster https://issues.redhat.com/browse/CRW-2677
		// The root cause is in the HTTP2 protocol support of the okttp3 library that is used by fabric8.kubernetes-client that is used by che-server
		// In the past, when che-server used Java 8, HTTP1 protocol was used. Now che-sever uses Java 11
		Http2Disable: strconv.FormatBool(true),
	}

	data.IdentityProviderUrl = identityProviderURL

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)

	}
	err = json.Unmarshal(out, &cheEnv)

	// k8s specific envs
	if !infrastructure.IsOpenShift() {
		k8sCheEnv := map[string]string{
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_FS__GROUP":     securityContextFsGroup,
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_RUN__AS__USER": securityContextRunAsUser,
			"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":                      ingressDomain,
			"CHE_INFRA_KUBERNETES_TLS__SECRET":                         tlsSecretName,
			"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON":           "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/$1\",\"nginx.ingress.kubernetes.io/ssl-redirect\": \"true\",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
			"CHE_INFRA_KUBERNETES_INGRESS_PATH__TRANSFORM":             "%s(.*)",
		}
		k8sCheEnv["CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S"] = "true"
		utils.AddMap(cheEnv, k8sCheEnv)
	}

	// Add TLS key and server certificate to properties since user workspaces is created in another
	// than Che server namespace, from where the Che TLS secret is not accessable
	if tlsSecretName != "" {
		cheTLSSecret := &corev1.Secret{}
		exists, err := deploy.GetNamespacedObject(ctx, tlsSecretName, cheTLSSecret)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("%s secret not found", tlsSecretName)
		} else {
			if _, exists := cheTLSSecret.Data["tls.key"]; !exists {
				return nil, fmt.Errorf("%s secret has no 'tls.key' key in data", tlsSecretName)
			}
			if _, exists := cheTLSSecret.Data["tls.crt"]; !exists {
				return nil, fmt.Errorf("%s secret has no 'tls.crt' key in data", tlsSecretName)
			}
			cheEnv["CHE_INFRA_KUBERNETES_TLS__KEY"] = string(cheTLSSecret.Data["tls.key"])
			cheEnv["CHE_INFRA_KUBERNETES_TLS__CERT"] = string(cheTLSSecret.Data["tls.crt"])
		}
	}

	utils.AddMap(cheEnv, ctx.CheCluster.Spec.Components.CheServer.ExtraProperties)

	s.updateUserClusterRoles(ctx, cheEnv)

	for _, oauthProvider := range []string{"bitbucket", "gitlab", constants.AzureDevOpsOAuth} {
		err := s.updateIntegrationServerEndpoints(ctx, cheEnv, oauthProvider)
		if err != nil {
			return nil, err
		}
	}

	return cheEnv, nil
}

func (s *CheServerReconciler) updateIntegrationServerEndpoints(ctx *chetypes.DeployContext, cheEnv map[string]string, oauthProvider string) error {
	secret, err := getOAuthConfig(ctx, oauthProvider)
	if secret == nil {
		return err
	}

	envName := fmt.Sprintf("CHE_INTEGRATION_%s_SERVER__ENDPOINTS", strings.ReplaceAll(strings.ToUpper(oauthProvider), "-", "_"))
	if err != nil {
		return err
	}

	if cheEnv[envName] != "" {
		cheEnv[envName] = secret.Annotations[constants.CheEclipseOrgScmServerEndpoint] + "," + cheEnv[envName]
	} else {
		cheEnv[envName] = secret.Annotations[constants.CheEclipseOrgScmServerEndpoint]
	}
	return nil
}

func GetCheConfigMapVersion(deployContext *chetypes.DeployContext) string {
	cheConfigMap := &corev1.ConfigMap{}
	exists, _ := deploy.GetNamespacedObject(deployContext, CheConfigMapName, cheConfigMap)
	if exists {
		return cheConfigMap.ResourceVersion
	}
	return ""
}

func (s *CheServerReconciler) updateUserClusterRoles(ctx *chetypes.DeployContext, cheEnv map[string]string) {
	userClusterRoles := strings.Join(s.getUserClusterRoles(ctx), ", ")

	for _, role := range strings.Split(cheEnv["CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"], ",") {
		role := strings.TrimSpace(role)
		if !strings.Contains(userClusterRoles, role) {
			userClusterRoles = userClusterRoles + ", " + role
		}
	}

	if ctx.CheCluster.Spec.DevEnvironments.User != nil {
		for _, role := range ctx.CheCluster.Spec.DevEnvironments.User.ClusterRoles {
			role := strings.TrimSpace(role)
			if !strings.Contains(userClusterRoles, role) {
				userClusterRoles = userClusterRoles + ", " + role
			}
		}
	}

	cheEnv["CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"] = userClusterRoles
}
