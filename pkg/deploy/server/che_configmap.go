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

	"github.com/eclipse/che-operator/pkg/deploy"

	"github.com/eclipse/che-operator/pkg/util"
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
	NamespaceAllowUserDefined              string `json:"CHE_INFRA_KUBERNETES_NAMESPACE_ALLOW__USER__DEFINED"`
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
}

func SyncCheConfigMapToCluster(deployContext *deploy.DeployContext) (*corev1.ConfigMap, error) {
	data, err := GetCheConfigMapData(deployContext)
	if err != nil {
		return nil, err
	}
	specConfigMap, err := deploy.GetSpecConfigMap(deployContext, CheConfigMapName, data, deploy.DefaultCheFlavor(deployContext.CheCluster))
	if err != nil {
		return nil, err
	}

	return deploy.SyncConfigMapToCluster(deployContext, specConfigMap)
}

// GetCheConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func GetCheConfigMapData(deployContext *deploy.DeployContext) (cheEnv map[string]string, err error) {
	cr := deployContext.CheCluster
	cheHost := cr.Spec.Server.CheHost
	keycloakURL := cr.Spec.Auth.IdentityProviderURL
	isOpenShift, isOpenshift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("Failed to get current infra: %s", err)
	}
	cheFlavor := deploy.DefaultCheFlavor(cr)
	infra := "kubernetes"
	if isOpenShift {
		infra = "openshift"
	}
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	defaultTargetNamespaceDefault := cr.Namespace // By default Che SA has right in the namespace where Che in installed ...
	if isOpenShift && util.IsOAuthEnabled(cr) {
		// ... But if the workspace is created under the openshift identity of the end-user,
		// Then we'll have rights to create any new namespace
		defaultTargetNamespaceDefault = "<username>-" + cheFlavor
		openShiftIdentityProviderId = "openshift-v3"
		if isOpenshift4 {
			openShiftIdentityProviderId = "openshift-v4"
		}
	}
	defaultTargetNamespace := util.GetValue(cr.Spec.Server.WorkspaceNamespaceDefault, defaultTargetNamespaceDefault)
	namespaceAllowUserDefined := strconv.FormatBool(cr.Spec.Server.AllowUserDefinedWorkspaceNamespaces)
	tlsSupport := cr.Spec.Server.TlsSupport
	protocol := "http"
	wsprotocol := "ws"
	if tlsSupport {
		protocol = "https"
		wsprotocol = "wss"
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

	ingressDomain := cr.Spec.K8s.IngressDomain
	tlsSecretName := cr.Spec.K8s.TlsSecretName
	securityContextFsGroup := util.GetValue(cr.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(cr.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(cr.Spec.Storage.PvcStrategy, deploy.DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(cr.Spec.Storage.PvcClaimSize, deploy.DefaultPvcClaimSize)
	workspacePvcStorageClassName := cr.Spec.Storage.WorkspacePVCStorageClassName

	defaultPVCJobsImage := deploy.DefaultPvcJobsImage(cr)
	pvcJobsImage := util.GetValue(cr.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !cr.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(cr.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName)
	chePostgresPort := util.GetValue(cr.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort)
	chePostgresDb := util.GetValue(cr.Spec.Database.ChePostgresDb, deploy.DefaultChePostgresDb)
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(cr.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	ingressStrategy := util.GetServerExposureStrategy(cr, deploy.DefaultServerExposureStrategy)
	ingressClass := util.GetValue(cr.Spec.K8s.IngressClass, deploy.DefaultIngressClass)
	devfileRegistryURL := cr.Status.DevfileRegistryURL
	pluginRegistryURL := cr.Status.PluginRegistryURL
	cheLogLevel := util.GetValue(cr.Spec.Server.CheLogLevel, deploy.DefaultCheLogLevel)
	cheDebug := util.GetValue(cr.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	cheMetrics := strconv.FormatBool(cr.Spec.Metrics.Enable)
	cheLabels := util.MapToKeyValuePairs(deploy.GetLabels(cr, deploy.DefaultCheFlavor(cr)))
	cheMultiUser := deploy.GetCheMultiUser(cr)
	workspaceExposure := deploy.GetSingleHostExposureType(cr)
	singleHostGatewayConfigMapLabels := labels.FormatLabels(util.GetMapValue(cr.Spec.Server.SingleHostGatewayConfigMapLabels, deploy.DefaultSingleHostGatewayConfigMapLabels))

	cheAPI := protocol + "://" + cheHost + "/api"
	var keycloakInternalURL, pluginRegistryInternalURL, devfileRegistryInternalURL, cheInternalAPI string

	if cr.Spec.Server.UseInternalClusterSVCNames && !cr.Spec.Auth.ExternalIdentityProvider {
		keycloakInternalURL = deployContext.InternalService.KeycloakHost
	} else {
		keycloakInternalURL = keycloakURL
	}

	if cr.Spec.Server.UseInternalClusterSVCNames && !deployContext.CheCluster.Spec.Server.ExternalDevfileRegistry {
		devfileRegistryInternalURL = deployContext.InternalService.DevfileRegistryHost
	} else {
		devfileRegistryInternalURL = devfileRegistryURL
	}

	if cr.Spec.Server.UseInternalClusterSVCNames && !deployContext.CheCluster.Spec.Server.ExternalPluginRegistry {
		pluginRegistryInternalURL = deployContext.InternalService.PluginRegistryHost
	} else {
		pluginRegistryInternalURL = pluginRegistryURL
	}

	if cr.Spec.Server.UseInternalClusterSVCNames {
		cheInternalAPI = deployContext.InternalService.CheHost + "/api"
	} else {
		cheInternalAPI = cheAPI
	}

	data := &CheConfigMap{
		CheMultiUser:                           cheMultiUser,
		CheHost:                                cheHost,
		ChePort:                                "8080",
		CheApi:                                 cheAPI,
		CheApiInternal:                         cheInternalAPI,
		CheWebSocketEndpoint:                   wsprotocol + "://" + cheHost + "/api/websocket",
		WebSocketEndpointMinor:                 wsprotocol + "://" + cheHost + "/api/websocket-minor",
		CheDebugServer:                         cheDebug,
		CheInfrastructureActive:                infra,
		CheInfraKubernetesServiceAccountName:   "che-workspace",
		DefaultTargetNamespace:                 defaultTargetNamespace,
		NamespaceAllowUserDefined:              namespaceAllowUserDefined,
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
		CheWorkspacePluginBrokerMetadataImage:  deploy.DefaultCheWorkspacePluginBrokerMetadataImage(cr),
		CheWorkspacePluginBrokerArtifactsImage: deploy.DefaultCheWorkspacePluginBrokerArtifactsImage(cr),
		CheServerSecureExposerJwtProxyImage:    deploy.DefaultCheServerSecureExposerJwtProxyImage(cr),
		CheJGroupsKubernetesLabels:             cheLabels,
		CheMetricsEnabled:                      cheMetrics,
		CheTrustedCABundlesConfigMap:           deploy.CheAllCACertsConfigMapName,
		ServerStrategy:                         ingressStrategy,
		WorkspaceExposure:                      workspaceExposure,
		SingleHostGatewayConfigMapLabels:       singleHostGatewayConfigMapLabels,
	}

	if cheMultiUser == "true" {
		data.KeycloakURL = keycloakURL + "/auth"
		data.KeycloakInternalURL = keycloakInternalURL + "/auth"
		data.KeycloakRealm = keycloakRealm
		data.KeycloakClientId = keycloakClientId
		data.DatabaseURL = "jdbc:postgresql://" + chePostgresHostName + ":" + chePostgresPort + "/" + chePostgresDb
		if len(cr.Spec.Database.ChePostgresSecret) < 1 {
			data.DbUserName = cr.Spec.Database.ChePostgresUser
			data.DbPassword = cr.Spec.Database.ChePostgresPassword
		}
	}

	out, err := json.Marshal(data)
	if err != nil {
		fmt.Println(err)

	}
	err = json.Unmarshal(out, &cheEnv)

	// k8s specific envs
	if !isOpenShift {
		k8sCheEnv := map[string]string{
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_FS__GROUP":     securityContextFsGroup,
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_RUN__AS__USER": securityContextRunAsUser,
			"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":                      ingressDomain,
			"CHE_INFRA_KUBERNETES_TLS__SECRET":                         tlsSecretName,
			"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON":           "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/$1\",\"nginx.ingress.kubernetes.io/ssl-redirect\": " + tls + ",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
			"CHE_INFRA_KUBERNETES_INGRESS_PATH__TRANSFORM":             "%s(.*)",
		}

		// Add TLS key and server certificate to properties when user workspaces should be created in another
		// than Che server namespace, from where the Che TLS secret is not accessable
		k8sDefaultNamespace := cr.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"]
		if (defaultTargetNamespace != "" && defaultTargetNamespace != cr.Namespace) ||
			(k8sDefaultNamespace != "" && k8sDefaultNamespace != cr.Namespace) {

			cheTLSSecret, err := deploy.GetClusterSecret(cr.Spec.K8s.TlsSecretName, cr.ObjectMeta.Namespace, deployContext.ClusterAPI)
			if err != nil {
				return nil, err
			}
			if cheTLSSecret == nil {
				return nil, fmt.Errorf("%s secret not found", cr.Spec.K8s.TlsSecretName)
			}
			if _, exists := cheTLSSecret.Data["tls.key"]; !exists {
				return nil, fmt.Errorf("%s secret has no 'tls.key' key in data", cr.Spec.K8s.TlsSecretName)
			}
			if _, exists := cheTLSSecret.Data["tls.crt"]; !exists {
				return nil, fmt.Errorf("%s secret has no 'tls.crt' key in data", cr.Spec.K8s.TlsSecretName)
			}
			k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__KEY"] = string(cheTLSSecret.Data["tls.key"])
			k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__CERT"] = string(cheTLSSecret.Data["tls.crt"])
		}

		addMap(cheEnv, k8sCheEnv)
	}

	addMap(cheEnv, cr.Spec.Server.CustomCheProperties)
	return cheEnv, nil
}

func GetCheConfigMapVersion(deployContext *deploy.DeployContext) string {
	cheConfigMap, _ := deploy.GetClusterConfigMap("che", deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if cheConfigMap != nil {
		return cheConfigMap.ResourceVersion
	}
	return ""
}
