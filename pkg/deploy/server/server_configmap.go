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
package server

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	deploytls "github.com/eclipse-che/che-operator/pkg/deploy/tls"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	CheConfigMapName = "che"
)

func addMap(a map[string]string, b map[string]string) {
	for k, v := range b {
		a[k] = v
	}
}

type CheConfigMap struct {
	CheHost                                string `json:"CHE_HOST"`
	CheMultiUser                           string `json:"CHE_MULTIUSER"`
	ChePort                                string `json:"CHE_PORT"`
	CheApi                                 string `json:"CHE_API"`
	CheApiInternal                         string `json:"CHE_API_INTERNAL"`
	CheWebSocketEndpoint                   string `json:"CHE_WEBSOCKET_ENDPOINT"`
	CheWebSocketInternalEndpoint           string `json:"CHE_WEBSOCKET_INTERNAL_ENDPOINT"`
	CheDebugServer                         string `json:"CHE_DEBUG_SERVER"`
	CheMetricsEnabled                      string `json:"CHE_METRICS_ENABLED"`
	CheInfrastructureActive                string `json:"CHE_INFRASTRUCTURE_ACTIVE"`
	CheInfraKubernetesServiceAccountName   string `json:"CHE_INFRA_KUBERNETES_SERVICE__ACCOUNT__NAME"`
	CheInfraKubernetesUserClusterRoles     string `json:"CHE_INFRA_KUBERNETES_USER__CLUSTER__ROLES"`
	DefaultTargetNamespace                 string `json:"CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"`
	PvcStrategy                            string `json:"CHE_INFRA_KUBERNETES_PVC_STRATEGY"`
	PvcClaimSize                           string `json:"CHE_INFRA_KUBERNETES_PVC_QUANTITY"`
	PvcJobsImage                           string `json:"CHE_INFRA_KUBERNETES_PVC_JOBS_IMAGE"`
	WorkspacePvcStorageClassName           string `json:"CHE_INFRA_KUBERNETES_PVC_STORAGE__CLASS__NAME"`
	PreCreateSubPaths                      string `json:"CHE_INFRA_KUBERNETES_PVC_PRECREATE__SUBPATHS"`
	TlsSupport                             string `json:"CHE_INFRA_OPENSHIFT_TLS__ENABLED"`
	K8STrustCerts                          string `json:"CHE_INFRA_KUBERNETES_TRUST__CERTS"`
	DatabaseURL                            string `json:"CHE_JDBC_URL,omitempty"`
	DbUserName                             string `json:"CHE_JDBC_USERNAME,omitempty"`
	DbPassword                             string `json:"CHE_JDBC_PASSWORD,omitempty"`
	CheLogLevel                            string `json:"CHE_LOG_LEVEL"`
	IdentityProviderUrl                    string `json:"CHE_OIDC_AUTH__SERVER__URL,omitempty"`
	IdentityProviderInternalURL            string `json:"CHE_OIDC_AUTH__INTERNAL__SERVER__URL,omitempty"`
	KeycloakRealm                          string `json:"CHE_KEYCLOAK_REALM,omitempty"`
	KeycloakClientId                       string `json:"CHE_KEYCLOAK_CLIENT__ID,omitempty"`
	OpenShiftIdentityProvider              string `json:"CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"`
	JavaOpts                               string `json:"JAVA_OPTS"`
	WorkspaceJavaOpts                      string `json:"CHE_WORKSPACE_JAVA__OPTIONS"`
	WorkspaceMavenOpts                     string `json:"CHE_WORKSPACE_MAVEN__OPTIONS"`
	WorkspaceProxyJavaOpts                 string `json:"CHE_WORKSPACE_HTTP__PROXY__JAVA__OPTIONS"`
	WorkspaceHttpProxy                     string `json:"CHE_WORKSPACE_HTTP__PROXY"`
	WorkspaceHttpsProxy                    string `json:"CHE_WORKSPACE_HTTPS__PROXY"`
	WorkspaceNoProxy                       string `json:"CHE_WORKSPACE_NO__PROXY"`
	PluginRegistryUrl                      string `json:"CHE_WORKSPACE_PLUGIN__REGISTRY__URL,omitempty"`
	PluginRegistryInternalUrl              string `json:"CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL,omitempty"`
	DevfileRegistryUrl                     string `json:"CHE_WORKSPACE_DEVFILE__REGISTRY__URL,omitempty"`
	DevfileRegistryInternalUrl             string `json:"CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL,omitempty"`
	CheWorkspacePluginBrokerMetadataImage  string `json:"CHE_WORKSPACE_PLUGIN__BROKER_METADATA_IMAGE,omitempty"`
	CheWorkspacePluginBrokerArtifactsImage string `json:"CHE_WORKSPACE_PLUGIN__BROKER_ARTIFACTS_IMAGE,omitempty"`
	CheServerSecureExposerJwtProxyImage    string `json:"CHE_SERVER_SECURE__EXPOSER_JWTPROXY_IMAGE,omitempty"`
	CheJGroupsKubernetesLabels             string `json:"KUBERNETES_LABELS,omitempty"`
	CheTrustedCABundlesConfigMap           string `json:"CHE_TRUSTED__CA__BUNDLES__CONFIGMAP,omitempty"`
	ServerStrategy                         string `json:"CHE_INFRA_KUBERNETES_SERVER__STRATEGY"`
	WorkspaceExposure                      string `json:"CHE_INFRA_KUBERNETES_SINGLEHOST_WORKSPACE_EXPOSURE"`
	SingleHostGatewayConfigMapLabels       string `json:"CHE_INFRA_KUBERNETES_SINGLEHOST_GATEWAY_CONFIGMAP__LABELS"`
	CheDevWorkspacesEnabled                string `json:"CHE_DEVWORKSPACES_ENABLED"`
	Http2Disable                           string `json:"HTTP2_DISABLE"`
}

// GetCheConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func (s *CheServerReconciler) getCheConfigMapData(ctx *deploy.DeployContext) (cheEnv map[string]string, err error) {
	cheHost := ctx.CheCluster.Spec.Server.CheHost
	identityProviderURL := ctx.CheCluster.Spec.Auth.IdentityProviderURL

	// Adds `/auth` for external identity providers.
	// If identity provide is deployed by operator then `/auth` is already added.
	if !ctx.CheCluster.IsNativeUserModeEnabled() &&
		ctx.CheCluster.Spec.Auth.ExternalIdentityProvider &&
		!strings.HasSuffix(identityProviderURL, "/auth") {
		if strings.HasSuffix(identityProviderURL, "/") {
			identityProviderURL = identityProviderURL + "auth"
		} else {
			identityProviderURL = identityProviderURL + "/auth"
		}
	}

	cheFlavor := deploy.DefaultCheFlavor(ctx.CheCluster)
	infra := "kubernetes"
	if util.IsOpenShift {
		infra = "openshift"
	}
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	if util.IsOpenShift && ctx.CheCluster.IsOpenShiftOAuthEnabled() {
		openShiftIdentityProviderId = "openshift-v3"
		if util.IsOpenShift4 {
			openShiftIdentityProviderId = "openshift-v4"
		}
	}
	tlsSupport := ctx.CheCluster.Spec.Server.TlsSupport
	protocol := "http"
	if tlsSupport {
		protocol = "https"
		tls = "true"
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

	ingressDomain := ctx.CheCluster.Spec.K8s.IngressDomain
	tlsSecretName := ctx.CheCluster.Spec.K8s.TlsSecretName
	securityContextFsGroup := util.GetValue(ctx.CheCluster.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(ctx.CheCluster.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(ctx.CheCluster.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(ctx.CheCluster.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	workspacePvcStorageClassName := ctx.CheCluster.Spec.Storage.WorkspacePVCStorageClassName

	defaultPVCJobsImage := deploy.DefaultPvcJobsImage(ctx.CheCluster)
	pvcJobsImage := util.GetValue(ctx.CheCluster.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !ctx.CheCluster.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
	chePostgresPort := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
	chePostgresDb := util.GetValue(ctx.CheCluster.Spec.Database.ChePostgresDb, deploy.DefaultChePostgresDb)
	keycloakRealm := util.GetValue(ctx.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(ctx.CheCluster.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	ingressStrategy := util.GetServerExposureStrategy(ctx.CheCluster)
	ingressClass := util.GetValue(ctx.CheCluster.Spec.K8s.IngressClass, deploy.DefaultIngressClass)

	// grab first the devfile registry url which is deployed by operator
	devfileRegistryURL := ctx.CheCluster.Status.DevfileRegistryURL

	// `Spec.Server.DevfileRegistryUrl` is deprecated in favor of `Server.ExternalDevfileRegistries`
	if ctx.CheCluster.Spec.Server.DevfileRegistryUrl != "" {
		devfileRegistryURL += " " + ctx.CheCluster.Spec.Server.DevfileRegistryUrl
	}
	for _, r := range ctx.CheCluster.Spec.Server.ExternalDevfileRegistries {
		if strings.Index(devfileRegistryURL, r.Url) == -1 {
			devfileRegistryURL += " " + r.Url
		}
	}
	devfileRegistryURL = strings.TrimSpace(devfileRegistryURL)

	pluginRegistryURL := ctx.CheCluster.Status.PluginRegistryURL
	cheLogLevel := util.GetValue(ctx.CheCluster.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	cheDebug := util.GetValue(ctx.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	cheMetrics := strconv.FormatBool(ctx.CheCluster.Spec.Metrics.Enable)
	cheLabels := util.MapToKeyValuePairs(deploy.GetLabels(ctx.CheCluster, deploy.DefaultCheFlavor(ctx.CheCluster)))
	workspaceExposure := deploy.GetSingleHostExposureType(ctx.CheCluster)
	singleHostGatewayConfigMapLabels := labels.FormatLabels(util.GetMapValue(ctx.CheCluster.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels))
	workspaceNamespaceDefault := util.GetWorkspaceNamespaceDefault(ctx.CheCluster)

	cheAPI := protocol + "://" + cheHost + "/api"
	var keycloakInternalURL, pluginRegistryInternalURL, devfileRegistryInternalURL, cheInternalAPI, webSocketInternalEndpoint string

	if !ctx.CheCluster.IsNativeUserModeEnabled() &&
		ctx.CheCluster.IsInternalClusterSVCNamesEnabled() &&
		!ctx.CheCluster.Spec.Auth.ExternalIdentityProvider {
		keycloakInternalURL = fmt.Sprintf("%s://%s.%s.svc:8080/auth", "http", deploy.IdentityProviderName, ctx.CheCluster.Namespace)
	}

	// If there is a devfile registry deployed by operator
	if ctx.CheCluster.IsInternalClusterSVCNamesEnabled() && !ctx.CheCluster.Spec.Server.ExternalDevfileRegistry {
		devfileRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080", deploy.DevfileRegistryName, ctx.CheCluster.Namespace)
	}

	if ctx.CheCluster.IsInternalClusterSVCNamesEnabled() && !ctx.CheCluster.Spec.Server.ExternalPluginRegistry {
		pluginRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080/v3", deploy.PluginRegistryName, ctx.CheCluster.Namespace)
	}

	if ctx.CheCluster.IsInternalClusterSVCNamesEnabled() {
		cheInternalAPI = fmt.Sprintf("http://%s.%s.svc:8080/api", deploy.CheServiceName, ctx.CheCluster.Namespace)
		webSocketInternalEndpoint = fmt.Sprintf("ws://%s.%s.svc:8080/api/websocket", deploy.CheServiceName, ctx.CheCluster.Namespace)
	}

	wsprotocol := "ws"
	if tlsSupport {
		wsprotocol = "wss"
	}
	webSocketEndpoint := wsprotocol + "://" + cheHost + "/api/websocket"

	cheWorkspaceServiceAccount := "che-workspace"
	cheUserClusterRoleNames := "NULL"
	if ctx.CheCluster.IsNativeUserModeEnabled() {
		cheWorkspaceServiceAccount = "NULL"
		cheUserClusterRoleNames = fmt.Sprintf("%s-cheworkspaces-clusterrole, %s-cheworkspaces-devworkspace-clusterrole", ctx.CheCluster.Namespace, ctx.CheCluster.Namespace)
	}

	data := &CheConfigMap{
		CheMultiUser:                           "true",
		CheHost:                                cheHost,
		ChePort:                                "8080",
		CheApi:                                 cheAPI,
		CheApiInternal:                         cheInternalAPI,
		CheWebSocketEndpoint:                   webSocketEndpoint,
		CheWebSocketInternalEndpoint:           webSocketInternalEndpoint,
		CheDebugServer:                         cheDebug,
		CheInfrastructureActive:                infra,
		CheInfraKubernetesServiceAccountName:   cheWorkspaceServiceAccount,
		CheInfraKubernetesUserClusterRoles:     cheUserClusterRoleNames,
		DefaultTargetNamespace:                 workspaceNamespaceDefault,
		PvcStrategy:                            pvcStrategy,
		PvcClaimSize:                           pvcClaimSize,
		WorkspacePvcStorageClassName:           workspacePvcStorageClassName,
		PvcJobsImage:                           pvcJobsImage,
		PreCreateSubPaths:                      preCreateSubPaths,
		TlsSupport:                             tls,
		K8STrustCerts:                          tls,
		CheLogLevel:                            cheLogLevel,
		OpenShiftIdentityProvider:              openShiftIdentityProviderId,
		JavaOpts:                               deploy.DefaultJavaOpts + " " + proxyJavaOpts,
		WorkspaceJavaOpts:                      deploy.DefaultWorkspaceJavaOpts + " " + proxyJavaOpts,
		WorkspaceMavenOpts:                     deploy.DefaultWorkspaceJavaOpts + " " + proxyJavaOpts,
		WorkspaceProxyJavaOpts:                 proxyJavaOpts,
		WorkspaceHttpProxy:                     ctx.Proxy.HttpProxy,
		WorkspaceHttpsProxy:                    ctx.Proxy.HttpsProxy,
		WorkspaceNoProxy:                       cheWorkspaceNoProxy,
		PluginRegistryUrl:                      pluginRegistryURL,
		PluginRegistryInternalUrl:              pluginRegistryInternalURL,
		DevfileRegistryUrl:                     devfileRegistryURL,
		DevfileRegistryInternalUrl:             devfileRegistryInternalURL,
		CheWorkspacePluginBrokerMetadataImage:  deploy.DefaultCheWorkspacePluginBrokerMetadataImage(ctx.CheCluster),
		CheWorkspacePluginBrokerArtifactsImage: deploy.DefaultCheWorkspacePluginBrokerArtifactsImage(ctx.CheCluster),
		CheServerSecureExposerJwtProxyImage:    deploy.DefaultCheServerSecureExposerJwtProxyImage(ctx.CheCluster),
		CheJGroupsKubernetesLabels:             cheLabels,
		CheMetricsEnabled:                      cheMetrics,
		CheTrustedCABundlesConfigMap:           deploytls.CheAllCACertsConfigMapName,
		ServerStrategy:                         ingressStrategy,
		WorkspaceExposure:                      workspaceExposure,
		SingleHostGatewayConfigMapLabels:       singleHostGatewayConfigMapLabels,
		CheDevWorkspacesEnabled:                strconv.FormatBool(ctx.CheCluster.Spec.DevWorkspace.Enable),
		// Disable HTTP2 protocol.
		// Fix issue with creating config maps on the cluster https://issues.redhat.com/browse/CRW-2677
		// The root cause is in the HTTP2 protocol support of the okttp3 library that is used by fabric8.kubernetes-client that is used by che-server
		// In the past, when che-server used Java 8, HTTP1 protocol was used. Now che-sever uses Java 11
		Http2Disable: strconv.FormatBool(true),
	}

	data.IdentityProviderUrl = identityProviderURL
	data.IdentityProviderInternalURL = keycloakInternalURL
	data.KeycloakRealm = keycloakRealm
	data.KeycloakClientId = keycloakClientId
	data.DatabaseURL = "jdbc:postgresql://" + chePostgresHostName + ":" + chePostgresPort + "/" + chePostgresDb
	if len(ctx.CheCluster.Spec.Database.ChePostgresSecret) < 1 {
		data.DbUserName = ctx.CheCluster.Spec.Database.ChePostgresUser
		data.DbPassword = ctx.CheCluster.Spec.Database.ChePostgresPassword
	}

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)

	}
	err = json.Unmarshal(out, &cheEnv)

	// k8s specific envs
	if !util.IsOpenShift {
		k8sCheEnv := map[string]string{
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_FS__GROUP":     securityContextFsGroup,
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_RUN__AS__USER": securityContextRunAsUser,
			"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":                      ingressDomain,
			"CHE_INFRA_KUBERNETES_TLS__SECRET":                         tlsSecretName,
			"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON":           "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/$1\",\"nginx.ingress.kubernetes.io/ssl-redirect\": " + tls + ",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
			"CHE_INFRA_KUBERNETES_INGRESS_PATH__TRANSFORM":             "%s(.*)",
		}

		if ctx.CheCluster.Spec.DevWorkspace.Enable {
			k8sCheEnv["CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S"] = "true"
		}

		// Add TLS key and server certificate to properties since user workspaces is created in another
		// than Che server namespace, from where the Che TLS secret is not accessable
		if ctx.CheCluster.Spec.K8s.TlsSecretName != "" {
			cheTLSSecret := &corev1.Secret{}
			exists, err := deploy.GetNamespacedObject(ctx, ctx.CheCluster.Spec.K8s.TlsSecretName, cheTLSSecret)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("%s secret not found", ctx.CheCluster.Spec.K8s.TlsSecretName)
			} else {
				if _, exists := cheTLSSecret.Data["tls.key"]; !exists {
					return nil, fmt.Errorf("%s secret has no 'tls.key' key in data", ctx.CheCluster.Spec.K8s.TlsSecretName)
				}
				if _, exists := cheTLSSecret.Data["tls.crt"]; !exists {
					return nil, fmt.Errorf("%s secret has no 'tls.crt' key in data", ctx.CheCluster.Spec.K8s.TlsSecretName)
				}
				k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__KEY"] = string(cheTLSSecret.Data["tls.key"])
				k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__CERT"] = string(cheTLSSecret.Data["tls.crt"])
			}
		}

		addMap(cheEnv, k8sCheEnv)
	}

	addMap(cheEnv, ctx.CheCluster.Spec.Server.CustomCheProperties)

	for _, oauthProvider := range []string{"bitbucket", "gitlab", "github"} {
		err := updateIntegrationServerEndpoints(ctx, cheEnv, oauthProvider)
		if err != nil {
			return nil, err
		}
	}

	return cheEnv, nil
}

func updateIntegrationServerEndpoints(ctx *deploy.DeployContext, cheEnv map[string]string, oauthProvider string) error {
	secret, err := getOAuthConfig(ctx, oauthProvider)
	if secret == nil {
		return err
	}

	envName := fmt.Sprintf("CHE_INTEGRATION_%s_SERVER__ENDPOINTS", strings.ToUpper(oauthProvider))
	if err != nil {
		return err
	}

	if cheEnv[envName] != "" {
		cheEnv[envName] = secret.Annotations[deploy.CheEclipseOrgScmServerEndpoint] + "," + cheEnv[envName]
	} else {
		cheEnv[envName] = secret.Annotations[deploy.CheEclipseOrgScmServerEndpoint]
	}
	return nil
}

func GetCheConfigMapVersion(deployContext *deploy.DeployContext) string {
	cheConfigMap := &corev1.ConfigMap{}
	exists, _ := deploy.GetNamespacedObject(deployContext, CheConfigMapName, cheConfigMap)
	if exists {
		return cheConfigMap.ResourceVersion
	}
	return ""
}
