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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
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
}

func SyncCheConfigMapToCluster(checluster *orgv1.CheCluster, proxy *Proxy, clusterAPI ClusterAPI) (*corev1.ConfigMap, error) {
	data := GetCheConfigMapData(checluster, proxy)
	specConfigMap, err := GetSpecConfigMap(checluster, CheConfigMapName, data, clusterAPI)
	if err != nil {
		return nil, err
	}

	return SyncConfigMapToCluster(checluster, specConfigMap, clusterAPI)
}

// GetConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func GetCheConfigMapData(cr *orgv1.CheCluster, proxy *Proxy) (cheEnv map[string]string) {
	cheHost := cr.Spec.Server.CheHost
	keycloakURL := cr.Spec.Auth.IdentityProviderURL
	isOpenShift, isOpenshift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("Failed to get current infra: %s", err)
	}
	cheFlavor := DefaultCheFlavor(cr)
	infra := "kubernetes"
	if isOpenShift {
		infra = "openshift"
	}
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	openshiftOAuth := cr.Spec.Auth.OpenShiftoAuth
	defaultTargetNamespaceDefault := cr.Namespace // By default Che SA has right in the namespace where Che in installed ...
	if openshiftOAuth && isOpenShift {
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
	cheWorkspaceNoProxy := proxy.NoProxy
	if proxy.HttpProxy != "" {
		if proxy.NoProxy == "" {
			cheWorkspaceNoProxy = os.Getenv("KUBERNETES_SERVICE_HOST")
		} else {
			cheWorkspaceNoProxy = cheWorkspaceNoProxy + "," + os.Getenv("KUBERNETES_SERVICE_HOST")
		}
		proxyJavaOpts, err = GenerateProxyJavaOpts(proxy, cheWorkspaceNoProxy)
		if err != nil {
			logrus.Errorf("Failed to generate java proxy options: %v", err)
		}
	}

	ingressDomain := cr.Spec.K8s.IngressDomain
	tlsSecretName := cr.Spec.K8s.TlsSecretName
	if tlsSupport && tlsSecretName == "" {
		tlsSecretName = "che-tls"
	}
	securityContextFsGroup := util.GetValue(cr.Spec.K8s.SecurityContextFsGroup, DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(cr.Spec.K8s.SecurityContextRunAsUser, DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(cr.Spec.Storage.PvcStrategy, DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(cr.Spec.Storage.PvcClaimSize, DefaultPvcClaimSize)
	workspacePvcStorageClassName := cr.Spec.Storage.WorkspacePVCStorageClassName

	defaultPVCJobsImage := DefaultPvcJobsImage(cr)
	pvcJobsImage := util.GetValue(cr.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !cr.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(cr.Spec.Database.ChePostgresHostName, DefaultChePostgresHostName)
	chePostgresPort := util.GetValue(cr.Spec.Database.ChePostgresPort, DefaultChePostgresPort)
	chePostgresDb := util.GetValue(cr.Spec.Database.ChePostgresDb, DefaultChePostgresDb)
	keycloakRealm := util.GetValue(cr.Spec.Auth.IdentityProviderRealm, cheFlavor)
	keycloakClientId := util.GetValue(cr.Spec.Auth.IdentityProviderClientId, cheFlavor+"-public")
	ingressStrategy := util.GetValue(cr.Spec.K8s.IngressStrategy, DefaultIngressStrategy)
	ingressClass := util.GetValue(cr.Spec.K8s.IngressClass, DefaultIngressClass)
	devfileRegistryUrl := cr.Status.DevfileRegistryURL
	pluginRegistryUrl := cr.Status.PluginRegistryURL
	cheLogLevel := util.GetValue(cr.Spec.Server.CheLogLevel, DefaultCheLogLevel)
	cheDebug := util.GetValue(cr.Spec.Server.CheDebug, DefaultCheDebug)
	cheMetrics := strconv.FormatBool(cr.Spec.Metrics.Enable)
	cheLabels := util.MapToKeyValuePairs(GetLabels(cr, DefaultCheFlavor(cr)))
	cheMultiUser := GetCheMultiUser(cr)

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
		WorkspaceHttpProxy:                     proxy.HttpProxy,
		WorkspaceHttpsProxy:                    proxy.HttpsProxy,
		WorkspaceNoProxy:                       cheWorkspaceNoProxy,
		PluginRegistryUrl:                      pluginRegistryUrl,
		DevfileRegistryUrl:                     devfileRegistryUrl,
		CheWorkspacePluginBrokerMetadataImage:  DefaultCheWorkspacePluginBrokerMetadataImage(cr),
		CheWorkspacePluginBrokerArtifactsImage: DefaultCheWorkspacePluginBrokerArtifactsImage(cr),
		CheServerSecureExposerJwtProxyImage:    DefaultCheServerSecureExposerJwtProxyImage(cr),
		CheJGroupsKubernetesLabels:             cheLabels,
		CheMetricsEnabled:                      cheMetrics,
	}

	if cheMultiUser == "true" {
		data.KeycloakURL = keycloakURL + "/auth"
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
	k8sCheEnv := map[string]string{
		"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_FS__GROUP":     securityContextFsGroup,
		"CHE_INFRA_KUBERNETES_POD_SECURITY__CONTEXT_RUN__AS__USER": securityContextRunAsUser,
		"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":                      ingressDomain,
		"CHE_INFRA_KUBERNETES_SERVER__STRATEGY":                    ingressStrategy,
		"CHE_INFRA_KUBERNETES_TLS__SECRET":                         tlsSecretName,
		"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON":           "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/$1\",\"nginx.ingress.kubernetes.io/ssl-redirect\": " + tls + ",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
		"CHE_INFRA_KUBERNETES_INGRESS_PATH__TRANSFORM":             "%s(.*)",
	}
	if !isOpenShift {
		addMap(cheEnv, k8sCheEnv)
	}

	addMap(cheEnv, cr.Spec.Server.CustomCheProperties)
	return cheEnv
}
