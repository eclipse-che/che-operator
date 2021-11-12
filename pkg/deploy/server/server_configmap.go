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
	KeycloakURL                            string `json:"CHE_KEYCLOAK_AUTH__SERVER__URL,omitempty"`
	KeycloakInternalURL                    string `json:"CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL,omitempty"`
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
}

// GetCheConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func (s *Server) getCheConfigMapData() (cheEnv map[string]string, err error) {
	cheHost := s.deployContext.CheCluster.Spec.Server.CheHost
	keycloakURL := s.deployContext.CheCluster.Spec.Auth.IdentityProviderURL

	// Adds `/auth` for external identity providers.
	// If identity provide is deployed by operator then `/auth` is already added.
	if s.deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider && !strings.HasSuffix(keycloakURL, "/auth") {
		if strings.HasSuffix(keycloakURL, "/") {
			keycloakURL = keycloakURL + "auth"
		} else {
			keycloakURL = keycloakURL + "/auth"
		}
	}

	cheFlavor := deploy.DefaultCheFlavor(s.deployContext.CheCluster)
	infra := "kubernetes"
	if util.IsOpenShift {
		infra = "openshift"
	}
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	if util.IsOpenShift && s.deployContext.CheCluster.IsOpenShiftOAuthEnabled() {
		openShiftIdentityProviderId = "openshift-v3"
		if util.IsOpenShift4 {
			openShiftIdentityProviderId = "openshift-v4"
		}
	}
	tlsSupport := s.deployContext.CheCluster.Spec.Server.TlsSupport
	protocol := "http"
	if tlsSupport {
		protocol = "https"
		tls = "true"
	}

	proxyJavaOpts := ""
	cheWorkspaceNoProxy := s.deployContext.Proxy.NoProxy
	if s.deployContext.Proxy.HttpProxy != "" {
		if s.deployContext.Proxy.NoProxy == "" {
			cheWorkspaceNoProxy = os.Getenv("KUBERNETES_SERVICE_HOST")
		} else {
			cheWorkspaceNoProxy = cheWorkspaceNoProxy + "," + os.Getenv("KUBERNETES_SERVICE_HOST")
		}
		proxyJavaOpts, err = deploy.GenerateProxyJavaOpts(s.deployContext.Proxy, cheWorkspaceNoProxy)
		if err != nil {
			logrus.Errorf("Failed to generate java proxy options: %v", err)
		}
	}

	ingressDomain := s.deployContext.CheCluster.Spec.K8s.IngressDomain
	tlsSecretName := s.deployContext.CheCluster.Spec.K8s.TlsSecretName
	securityContextFsGroup := util.GetValue(s.deployContext.CheCluster.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(s.deployContext.CheCluster.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(s.deployContext.CheCluster.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(s.deployContext.CheCluster.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	workspacePvcStorageClassName := s.deployContext.CheCluster.Spec.Storage.WorkspacePVCStorageClassName

	defaultPVCJobsImage := deploy.DefaultPvcJobsImage(s.deployContext.CheCluster)
	pvcJobsImage := util.GetValue(s.deployContext.CheCluster.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !s.deployContext.CheCluster.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(s.deployContext.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
	chePostgresPort := util.GetValue(s.deployContext.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
	chePostgresDb := util.GetValue(s.deployContext.CheCluster.Spec.Database.ChePostgresDb, deploy.DefaultChePostgresDb)
	keycloakRealm := util.GetValue(s.deployContext.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(s.deployContext.CheCluster.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	ingressStrategy := util.GetServerExposureStrategy(s.deployContext.CheCluster)
	ingressClass := util.GetValue(s.deployContext.CheCluster.Spec.K8s.IngressClass, deploy.DefaultIngressClass)

	// grab first the devfile registry url which is deployed by operator
	devfileRegistryURL := s.deployContext.CheCluster.Status.DevfileRegistryURL

	// `Spec.Server.DevfileRegistryUrl` is deprecated in favor of `Server.ExternalDevfileRegistries`
	if s.deployContext.CheCluster.Spec.Server.DevfileRegistryUrl != "" {
		devfileRegistryURL += " " + s.deployContext.CheCluster.Spec.Server.DevfileRegistryUrl
	}
	for _, r := range s.deployContext.CheCluster.Spec.Server.ExternalDevfileRegistries {
		if strings.Index(devfileRegistryURL, r.Url) == -1 {
			devfileRegistryURL += " " + r.Url
		}
	}
	devfileRegistryURL = strings.TrimSpace(devfileRegistryURL)

	pluginRegistryURL := s.deployContext.CheCluster.Status.PluginRegistryURL
	cheLogLevel := util.GetValue(s.deployContext.CheCluster.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	cheDebug := util.GetValue(s.deployContext.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	cheMetrics := strconv.FormatBool(s.deployContext.CheCluster.Spec.Metrics.Enable)
	cheLabels := util.MapToKeyValuePairs(deploy.GetLabels(s.deployContext.CheCluster, deploy.DefaultCheFlavor(s.deployContext.CheCluster)))
	workspaceExposure := deploy.GetSingleHostExposureType(s.deployContext.CheCluster)
	singleHostGatewayConfigMapLabels := labels.FormatLabels(util.GetMapValue(s.deployContext.CheCluster.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels))
	workspaceNamespaceDefault := util.GetWorkspaceNamespaceDefault(s.deployContext.CheCluster)

	cheAPI := protocol + "://" + cheHost + "/api"
	var keycloakInternalURL, pluginRegistryInternalURL, devfileRegistryInternalURL, cheInternalAPI, webSocketInternalEndpoint string

	if s.deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() && !s.deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider {
		keycloakInternalURL = fmt.Sprintf("%s://%s.%s.svc:8080/auth", "http", deploy.IdentityProviderName, s.deployContext.CheCluster.Namespace)
	}

	// If there is a devfile registry deployed by operator
	if s.deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() && !s.deployContext.CheCluster.Spec.Server.ExternalDevfileRegistry {
		devfileRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080", deploy.DevfileRegistryName, s.deployContext.CheCluster.Namespace)
	}

	if s.deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() && !s.deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		pluginRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080/v3", deploy.PluginRegistryName, s.deployContext.CheCluster.Namespace)
	}

	if s.deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() {
		cheInternalAPI = fmt.Sprintf("http://%s.%s.svc:8080/api", deploy.CheServiceName, s.deployContext.CheCluster.Namespace)
		webSocketInternalEndpoint = fmt.Sprintf("ws://%s.%s.svc:8080/api/websocket", deploy.CheServiceName, s.deployContext.CheCluster.Namespace)
	}

	wsprotocol := "ws"
	if tlsSupport {
		wsprotocol = "wss"
	}
	webSocketEndpoint := wsprotocol + "://" + cheHost + "/api/websocket"

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
		CheInfraKubernetesServiceAccountName:   "che-workspace",
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
		WorkspaceHttpProxy:                     s.deployContext.Proxy.HttpProxy,
		WorkspaceHttpsProxy:                    s.deployContext.Proxy.HttpsProxy,
		WorkspaceNoProxy:                       cheWorkspaceNoProxy,
		PluginRegistryUrl:                      pluginRegistryURL,
		PluginRegistryInternalUrl:              pluginRegistryInternalURL,
		DevfileRegistryUrl:                     devfileRegistryURL,
		DevfileRegistryInternalUrl:             devfileRegistryInternalURL,
		CheWorkspacePluginBrokerMetadataImage:  deploy.DefaultCheWorkspacePluginBrokerMetadataImage(s.deployContext.CheCluster),
		CheWorkspacePluginBrokerArtifactsImage: deploy.DefaultCheWorkspacePluginBrokerArtifactsImage(s.deployContext.CheCluster),
		CheServerSecureExposerJwtProxyImage:    deploy.DefaultCheServerSecureExposerJwtProxyImage(s.deployContext.CheCluster),
		CheJGroupsKubernetesLabels:             cheLabels,
		CheMetricsEnabled:                      cheMetrics,
		CheTrustedCABundlesConfigMap:           deploytls.CheAllCACertsConfigMapName,
		ServerStrategy:                         ingressStrategy,
		WorkspaceExposure:                      workspaceExposure,
		SingleHostGatewayConfigMapLabels:       singleHostGatewayConfigMapLabels,
		CheDevWorkspacesEnabled:                strconv.FormatBool(s.deployContext.CheCluster.Spec.DevWorkspace.Enable),
	}

	data.KeycloakURL = keycloakURL
	data.KeycloakInternalURL = keycloakInternalURL
	data.KeycloakRealm = keycloakRealm
	data.KeycloakClientId = keycloakClientId
	data.DatabaseURL = "jdbc:postgresql://" + chePostgresHostName + ":" + chePostgresPort + "/" + chePostgresDb
	if len(s.deployContext.CheCluster.Spec.Database.ChePostgresSecret) < 1 {
		data.DbUserName = s.deployContext.CheCluster.Spec.Database.ChePostgresUser
		data.DbPassword = s.deployContext.CheCluster.Spec.Database.ChePostgresPassword
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

		if s.deployContext.CheCluster.Spec.DevWorkspace.Enable {
			k8sCheEnv["CHE_INFRA_KUBERNETES_ENABLE__UNSUPPORTED__K8S"] = "true"
		}

		// Add TLS key and server certificate to properties since user workspaces is created in another
		// than Che server namespace, from where the Che TLS secret is not accessable
		if s.deployContext.CheCluster.Spec.K8s.TlsSecretName != "" {
			cheTLSSecret := &corev1.Secret{}
			exists, err := deploy.GetNamespacedObject(s.deployContext, s.deployContext.CheCluster.Spec.K8s.TlsSecretName, cheTLSSecret)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("%s secret not found", s.deployContext.CheCluster.Spec.K8s.TlsSecretName)
			} else {
				if _, exists := cheTLSSecret.Data["tls.key"]; !exists {
					return nil, fmt.Errorf("%s secret has no 'tls.key' key in data", s.deployContext.CheCluster.Spec.K8s.TlsSecretName)
				}
				if _, exists := cheTLSSecret.Data["tls.crt"]; !exists {
					return nil, fmt.Errorf("%s secret has no 'tls.crt' key in data", s.deployContext.CheCluster.Spec.K8s.TlsSecretName)
				}
				k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__KEY"] = string(cheTLSSecret.Data["tls.key"])
				k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__CERT"] = string(cheTLSSecret.Data["tls.crt"])
			}
		}

		addMap(cheEnv, k8sCheEnv)
	}

	addMap(cheEnv, s.deployContext.CheCluster.Spec.Server.CustomCheProperties)

	err = setBitbucketEndpoints(s.deployContext, cheEnv)
	if err != nil {
		return nil, err
	}

	return cheEnv, nil
}

func setBitbucketEndpoints(deployContext *deploy.DeployContext, cheEnv map[string]string) error {
	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "bitbucket",
	})

	if err != nil {
		return err
	} else if len(secrets) == 1 {
		serverEndpoint := secrets[0].Annotations[deploy.CheEclipseOrgScmServerEndpoint]
		endpoints, exists := cheEnv["CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS"]
		if exists {
			cheEnv["CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS"] = endpoints + "," + serverEndpoint
		} else {
			cheEnv["CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS"] = serverEndpoint
		}
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
