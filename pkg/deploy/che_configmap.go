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
package deploy

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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
	DevfileRegistryUrl                     string `json:"CHE_WORKSPACE_DEVFILE__REGISTRY__URL,omitempty"`
	WebSocketEndpointMinor                 string `json:"CHE_WEBSOCKET_ENDPOINT__MINOR"`
	CheWorkspacePluginBrokerMetadataImage  string `json:"CHE_WORKSPACE_PLUGIN__BROKER_METADATA_IMAGE,omitempty"`
	CheWorkspacePluginBrokerArtifactsImage string `json:"CHE_WORKSPACE_PLUGIN__BROKER_ARTIFACTS_IMAGE,omitempty"`
	CheServerSecureExposerJwtProxyImage    string `json:"CHE_SERVER_SECURE__EXPOSER_JWTPROXY_IMAGE,omitempty"`
	CheJGroupsKubernetesLabels             string `json:"KUBERNETES_LABELS,omitempty"`
	CheTrustedCABundlesConfigMap           string `json:"CHE_TRUSTED__CA__BUNDLES__CONFIGMAP,omitempty"`
}

func SyncCheConfigMapToCluster(deployContext *DeployContext) (*corev1.ConfigMap, error) {
	data, err := GetCheConfigMapData(deployContext)
	if err != nil {
		return nil, err
	}
	specConfigMap, err := GetSpecConfigMap(deployContext, CheConfigMapName, data)
	if err != nil {
		return nil, err
	}

	return SyncConfigMapToCluster(deployContext, specConfigMap)
}

// GetCheConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func GetCheConfigMapData(deployContext *DeployContext) (cheEnv map[string]string, err error) {
	cheHost := deployContext.CheCluster.Spec.Server.CheHost
	keycloakURL := deployContext.CheCluster.Spec.Auth.IdentityProviderURL
	isOpenShift, isOpenshift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("Failed to get current infra: %s", err)
	}
	cheFlavor := DefaultCheFlavor(deployContext.CheCluster)
	infra := "kubernetes"
	if isOpenShift {
		infra = "openshift"
	}
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	openshiftOAuth := deployContext.CheCluster.Spec.Auth.OpenShiftoAuth
	defaultTargetNamespaceDefault := deployContext.CheCluster.Namespace // By default Che SA has right in the namespace where Che in installed ...
	if openshiftOAuth && isOpenShift {
		// ... But if the workspace is created under the openshift identity of the end-user,
		// Then we'll have rights to create any new namespace
		defaultTargetNamespaceDefault = "<username>-" + cheFlavor
		openShiftIdentityProviderId = "openshift-v3"
		if isOpenshift4 {
			openShiftIdentityProviderId = "openshift-v4"
		}
	}
	defaultTargetNamespace := util.GetValue(deployContext.CheCluster.Spec.Server.WorkspaceNamespaceDefault, defaultTargetNamespaceDefault)
	namespaceAllowUserDefined := strconv.FormatBool(deployContext.CheCluster.Spec.Server.AllowUserDefinedWorkspaceNamespaces)
	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
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
		proxyJavaOpts, err = GenerateProxyJavaOpts(deployContext.Proxy, cheWorkspaceNoProxy)
		if err != nil {
			logrus.Errorf("Failed to generate java proxy options: %v", err)
		}
	}

	ingressDomain := deployContext.CheCluster.Spec.K8s.IngressDomain
	tlsSecretName := deployContext.CheCluster.Spec.K8s.TlsSecretName
	if tlsSupport && tlsSecretName == "" {
		tlsSecretName = "che-tls"
	}
	securityContextFsGroup := util.GetValue(deployContext.CheCluster.Spec.K8s.SecurityContextFsGroup, DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(deployContext.CheCluster.Spec.K8s.SecurityContextRunAsUser, DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcStrategy, DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcClaimSize, DefaultPvcClaimSize)
	workspacePvcStorageClassName := deployContext.CheCluster.Spec.Storage.WorkspacePVCStorageClassName

	defaultPVCJobsImage := DefaultPvcJobsImage(deployContext.CheCluster)
	pvcJobsImage := util.GetValue(deployContext.CheCluster.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !deployContext.CheCluster.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresHostName, DefaultChePostgresHostName)
	chePostgresPort := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresPort, DefaultChePostgresPort)
	chePostgresDb := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresDb, DefaultChePostgresDb)
	keycloakRealm := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	ingressStrategy := util.GetValue(deployContext.CheCluster.Spec.K8s.IngressStrategy, DefaultIngressStrategy)
	ingressClass := util.GetValue(deployContext.CheCluster.Spec.K8s.IngressClass, DefaultIngressClass)
	devfileRegistryUrl := deployContext.CheCluster.Status.DevfileRegistryURL
	pluginRegistryUrl := deployContext.CheCluster.Status.PluginRegistryURL
	cheLogLevel := util.GetValue(deployContext.CheCluster.Spec.Server.CheLogLevel, DefaultCheLogLevel)
	cheDebug := util.GetValue(deployContext.CheCluster.Spec.Server.CheDebug, DefaultCheDebug)
	cheMetrics := strconv.FormatBool(deployContext.CheCluster.Spec.Metrics.Enable)
	cheLabels := util.MapToKeyValuePairs(GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster)))
	cheMultiUser := GetCheMultiUser(deployContext.CheCluster)

	data := &CheConfigMap{
		CheMultiUser:                           cheMultiUser,
		CheHost:                                cheHost,
		ChePort:                                "8080",
		CheApi:                                 protocol + "://" + cheHost + "/api",
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
		JavaOpts:                               DefaultJavaOpts + " " + proxyJavaOpts,
		WorkspaceJavaOpts:                      DefaultWorkspaceJavaOpts + " " + proxyJavaOpts,
		WorkspaceMavenOpts:                     DefaultWorkspaceJavaOpts + " " + proxyJavaOpts,
		WorkspaceProxyJavaOpts:                 proxyJavaOpts,
		WorkspaceHttpProxy:                     deployContext.Proxy.HttpProxy,
		WorkspaceHttpsProxy:                    deployContext.Proxy.HttpsProxy,
		WorkspaceNoProxy:                       cheWorkspaceNoProxy,
		PluginRegistryUrl:                      pluginRegistryUrl,
		DevfileRegistryUrl:                     devfileRegistryUrl,
		CheWorkspacePluginBrokerMetadataImage:  DefaultCheWorkspacePluginBrokerMetadataImage(deployContext.CheCluster),
		CheWorkspacePluginBrokerArtifactsImage: DefaultCheWorkspacePluginBrokerArtifactsImage(deployContext.CheCluster),
		CheServerSecureExposerJwtProxyImage:    DefaultCheServerSecureExposerJwtProxyImage(deployContext.CheCluster),
		CheJGroupsKubernetesLabels:             cheLabels,
		CheMetricsEnabled:                      cheMetrics,
		CheTrustedCABundlesConfigMap:           deployContext.CheCluster.Spec.Server.ServerTrustStoreConfigMapName,
	}

	if cheMultiUser == "true" {
		data.KeycloakURL = keycloakURL + "/auth"
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
	if !isOpenShift {
		k8sCheEnv := map[string]string{
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_FS__GROUP":     securityContextFsGroup,
			"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_RUN__AS__USER": securityContextRunAsUser,
			"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":                      ingressDomain,
			"CHE_INFRA_KUBERNETES_SERVER__STRATEGY":                    ingressStrategy,
			"CHE_INFRA_KUBERNETES_TLS__SECRET":                         tlsSecretName,
			"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON":           "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/$1\",\"nginx.ingress.kubernetes.io/ssl-redirect\": " + tls + ",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
			"CHE_INFRA_KUBERNETES_INGRESS_PATH__TRANSFORM":             "%s(.*)",
		}
		// Add TLS key and server certificate to properties when user workspaces should be created in another
		// than Che server namespace, from where the Che TLS secret is not accesible.
		if _, keyExists := deployContext.CheCluster.Spec.Server.CustomCheProperties["CHE_INFRA_KUBERNETES_NAMESPACE_DEFAULT"]; keyExists {
			cheTLSSecret, err := GetClusterSecret(deployContext.CheCluster.Spec.K8s.TlsSecretName, deployContext.CheCluster.ObjectMeta.Namespace, deployContext.ClusterAPI)
			if err != nil {
				return nil, err
			}
			k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__KEY"] = string(cheTLSSecret.Data["tls.key"])
			k8sCheEnv["CHE_INFRA_KUBERNETES_TLS__CERT"] = string(cheTLSSecret.Data["tls.crt"])
		}

		addMap(cheEnv, k8sCheEnv)
	}

	addMap(cheEnv, deployContext.CheCluster.Spec.Server.CustomCheProperties)
	return cheEnv, nil
}
