//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	WebSocketEndpointMinor                 string `json:"CHE_WEBSOCKET_ENDPOINT__MINOR"`
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

func SyncCheConfigMapToCluster(deployContext *deploy.DeployContext) (bool, error) {
	data, err := GetCheConfigMapData(deployContext)
	if err != nil {
		return false, err
	}
	return deploy.SyncConfigMapDataToCluster(deployContext, CheConfigMapName, data, deploy.DefaultCheFlavor(deployContext.CheCluster))
}

// GetCheConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func GetCheConfigMapData(deployContext *deploy.DeployContext) (cheEnv map[string]string, err error) {
	cheHost := deployContext.CheCluster.Spec.Server.CheHost
	keycloakURL := deployContext.CheCluster.Spec.Auth.IdentityProviderURL

	// Adds `/auth` for external identity providers.
	// If identity provide is deployed by operator then `/auth` is already added.
	if deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider && !strings.HasSuffix(keycloakURL, "/auth") {
		if strings.HasSuffix(keycloakURL, "/") {
			keycloakURL = keycloakURL + "auth"
		} else {
			keycloakURL = keycloakURL + "/auth"
		}
	}

	if err != nil {
		logrus.Errorf("Failed to get current infra: %s", err)
	}
	cheFlavor := deploy.DefaultCheFlavor(deployContext.CheCluster)
	infra := "kubernetes"
	if util.IsOpenShift {
		infra = "openshift"
	}
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	if util.IsOpenShift && util.IsOAuthEnabled(deployContext.CheCluster) {
		openShiftIdentityProviderId = "openshift-v3"
		if util.IsOpenShift4 {
			openShiftIdentityProviderId = "openshift-v4"
		}
	}
	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
	protocol := "http"
	if tlsSupport {
		protocol = "https"
		tls = "true"
	}

	proxyJavaOpts := ""
	cheWorkspaceNoProxy := deployContext.Proxy.NoProxy
	if deployContext.Proxy.HttpProxy != "" {
		if deployContext.Proxy.NoProxy == "" {
			cheWorkspaceNoProxy = os.Getenv("KUBERNETES_SERVICE_HOST")
		} else {
			cheWorkspaceNoProxy = cheWorkspaceNoProxy + "," + os.Getenv("KUBERNETES_SERVICE_HOST")
		}
		proxyJavaOpts, err = deploy.GenerateProxyJavaOpts(deployContext.Proxy, cheWorkspaceNoProxy)
		if err != nil {
			logrus.Errorf("Failed to generate java proxy options: %v", err)
		}
	}

	ingressDomain := deployContext.CheCluster.Spec.K8s.IngressDomain
	tlsSecretName := deployContext.CheCluster.Spec.K8s.TlsSecretName
	securityContextFsGroup := util.GetValue(deployContext.CheCluster.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(deployContext.CheCluster.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	workspacePvcStorageClassName := deployContext.CheCluster.Spec.Storage.WorkspacePVCStorageClassName

	defaultPVCJobsImage := deploy.DefaultPvcJobsImage(deployContext.CheCluster)
	pvcJobsImage := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !deployContext.CheCluster.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
	chePostgresPort := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
	chePostgresDb := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresDb, deploy.DefaultChePostgresDb)
	keycloakRealm := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	ingressStrategy := util.GetServerExposureStrategy(deployContext.CheCluster)
	ingressClass := util.GetValue(deployContext.CheCluster.Spec.K8s.IngressClass, deploy.DefaultIngressClass)
	devfileRegistryURL := deployContext.CheCluster.Status.DevfileRegistryURL
	pluginRegistryURL := deployContext.CheCluster.Status.PluginRegistryURL
	cheLogLevel := util.GetValue(deployContext.CheCluster.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	cheDebug := util.GetValue(deployContext.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	cheMetrics := strconv.FormatBool(deployContext.CheCluster.Spec.Metrics.Enable)
	cheLabels := util.MapToKeyValuePairs(deploy.GetLabels(deployContext.CheCluster, deploy.DefaultCheFlavor(deployContext.CheCluster)))
	cheMultiUser := deploy.GetCheMultiUser(deployContext.CheCluster)
	workspaceExposure := deploy.GetSingleHostExposureType(deployContext.CheCluster)
	singleHostGatewayConfigMapLabels := labels.FormatLabels(util.GetMapValue(deployContext.CheCluster.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels))
	workspaceNamespaceDefault := util.GetWorkspaceNamespaceDefault(deployContext.CheCluster)

	cheAPI := protocol + "://" + cheHost + "/api"
	var keycloakInternalURL, pluginRegistryInternalURL, devfileRegistryInternalURL, cheInternalAPI, webSocketEndpoint, webSocketEndpointMinor string

	if deployContext.CheCluster.Spec.Server.UseInternalClusterSVCNames && !deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider {
		keycloakInternalURL = fmt.Sprintf("%s://%s.%s.svc:8080/auth", "http", deploy.IdentityProviderName, deployContext.CheCluster.Namespace)
	} else {
		keycloakInternalURL = keycloakURL
	}

	if deployContext.CheCluster.Spec.Server.UseInternalClusterSVCNames && !deployContext.CheCluster.Spec.Server.ExternalDevfileRegistry {
		devfileRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080", deploy.DevfileRegistryName, deployContext.CheCluster.Namespace)
	} else {
		devfileRegistryInternalURL = devfileRegistryURL
	}

	if deployContext.CheCluster.Spec.Server.UseInternalClusterSVCNames && !deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		pluginRegistryInternalURL = fmt.Sprintf("http://%s.%s.svc:8080/v3", deploy.PluginRegistryName, deployContext.CheCluster.Namespace)
	} else {
		pluginRegistryInternalURL = pluginRegistryURL
	}

	if deployContext.CheCluster.Spec.Server.UseInternalClusterSVCNames {
		cheInternalAPI = fmt.Sprintf("http://%s.%s.svc:8080/api", deploy.CheServiceName, deployContext.CheCluster.Namespace)
		webSocketEndpoint = fmt.Sprintf("ws://%s.%s.svc:8080/api/websocket", deploy.CheServiceName, deployContext.CheCluster.Namespace)
		webSocketEndpointMinor = fmt.Sprintf("ws://%s.%s.svc:8080/api/websocket-minor", deploy.CheServiceName, deployContext.CheCluster.Namespace)
	} else {
		cheInternalAPI = cheAPI

		wsprotocol := "ws"

		if tlsSupport {
			wsprotocol = "wss"
		}

		webSocketEndpoint = wsprotocol + "://" + cheHost + "/api/websocket"
		webSocketEndpointMinor = wsprotocol + "://" + cheHost + "/api/websocket-minor"
	}

	data := &CheConfigMap{
		CheMultiUser:                           cheMultiUser,
		CheHost:                                cheHost,
		ChePort:                                "8080",
		CheApi:                                 cheAPI,
		CheApiInternal:                         cheInternalAPI,
		CheWebSocketEndpoint:                   webSocketEndpoint,
		WebSocketEndpointMinor:                 webSocketEndpointMinor,
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
		WorkspaceHttpProxy:                     deployContext.Proxy.HttpProxy,
		WorkspaceHttpsProxy:                    deployContext.Proxy.HttpsProxy,
		WorkspaceNoProxy:                       cheWorkspaceNoProxy,
		PluginRegistryUrl:                      pluginRegistryURL,
		PluginRegistryInternalUrl:              pluginRegistryInternalURL,
		DevfileRegistryUrl:                     devfileRegistryURL,
		DevfileRegistryInternalUrl:             devfileRegistryInternalURL,
		CheWorkspacePluginBrokerMetadataImage:  deploy.DefaultCheWorkspacePluginBrokerMetadataImage(deployContext.CheCluster),
		CheWorkspacePluginBrokerArtifactsImage: deploy.DefaultCheWorkspacePluginBrokerArtifactsImage(deployContext.CheCluster),
		CheServerSecureExposerJwtProxyImage:    deploy.DefaultCheServerSecureExposerJwtProxyImage(deployContext.CheCluster),
		CheJGroupsKubernetesLabels:             cheLabels,
		CheMetricsEnabled:                      cheMetrics,
		CheTrustedCABundlesConfigMap:           deploy.CheAllCACertsConfigMapName,
		ServerStrategy:                         ingressStrategy,
		WorkspaceExposure:                      workspaceExposure,
		SingleHostGatewayConfigMapLabels:       singleHostGatewayConfigMapLabels,
		CheDevWorkspacesEnabled:                strconv.FormatBool(deployContext.CheCluster.Spec.DevWorkspace.Enable),
	}

	if cheMultiUser == "true" {
		data.KeycloakURL = keycloakURL
		data.KeycloakInternalURL = keycloakInternalURL
		data.KeycloakRealm = keycloakRealm
		data.KeycloakClientId = keycloakClientId
		data.DatabaseURL = "jdbc:postgresql://" + chePostgresHostName + ":" + chePostgresPort + "/" + chePostgresDb
		if len(deployContext.CheCluster.Spec.Database.ChePostgresSecret) < 1 {
			data.DbUserName = deployContext.CheCluster.Spec.Database.ChePostgresUser
			data.DbPassword = deployContext.CheCluster.Spec.Database.ChePostgresPassword
		}
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

		// Add TLS key and server certificate to properties since user workspaces is created in another
		// than Che server namespace, from where the Che TLS secret is not accessable
		if deployContext.CheCluster.Spec.K8s.TlsSecretName != "" {
			cheTLSSecret := &corev1.Secret{}
			exists, err := deploy.GetNamespacedObject(deployContext, deployContext.CheCluster.Spec.K8s.TlsSecretName, cheTLSSecret)
			if err != nil {
				return nil, err
			}
			if !exists {
				return nil, fmt.Errorf("%s secret not found", deployContext.CheCluster.Spec.K8s.TlsSecretName)
			} else {
				if _, exists := cheTLSSecret.Data["tls.key"]; !exists {
					return nil, fmt.Errorf("%s secret has no 'tls.key' key in data", deployContext.CheCluster.Spec.K8s.TlsSecretName)
				}
				if _, exists := cheTLSSecret.Data["tls.crt"]; !exists {
					return nil, fmt.Errorf("%s secret has no 'tls.crt' key in data", deployContext.CheCluster.Spec.K8s.TlsSecretName)
				}
				k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__KEY"] = string(cheTLSSecret.Data["tls.key"])
				k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__CERT"] = string(cheTLSSecret.Data["tls.crt"])
			}
		}

		addMap(cheEnv, k8sCheEnv)
	}

	addMap(cheEnv, deployContext.CheCluster.Spec.Server.CustomCheProperties)

	// Update BitBucket endpoints
	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "bitbucket",
	})
	if err != nil {
		return nil, err
	} else if len(secrets) == 1 {
		serverEndpoint := secrets[0].Annotations[deploy.CheEclipseOrgScmServerEndpoint]
		endpoints, exists := cheEnv["CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS"]
		if exists {
			cheEnv["CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS"] = endpoints + "," + serverEndpoint
		} else {
			cheEnv["CHE_INTEGRATION_BITBUCKET_SERVER__ENDPOINTS"] = serverEndpoint
		}
	}

	return cheEnv, nil
}

func GetCheConfigMapVersion(deployContext *deploy.DeployContext) string {
	cheConfigMap := &corev1.ConfigMap{}
	exists, _ := deploy.GetNamespacedObject(deployContext, CheConfigMapName, cheConfigMap)
	if exists {
		return cheConfigMap.ResourceVersion
	}
	return ""
}
