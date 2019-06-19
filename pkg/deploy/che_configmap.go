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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

func addMap(a map[string]string, b map[string]string) {
	for k, v := range b {
		a[k] = v
	}
}

type CheConfigMap struct {
	CheHost                      string `json:"CHE_HOST"`
	CheMultiUser                 string `json:"CHE_MULTIUSER"`
	ChePort                      string `json:"CHE_PORT"`
	CheApi                       string `json:"CHE_API"`
	CheWebSocketEndpoint         string `json:"CHE_WEBSOCKET_ENDPOINT"`
	CheDebugServer               string `json:"CHE_DEBUG_SERVER"`
	CheInfrastructureActive      string `json:"CHE_INFRASTRUCTURE_ACTIVE"`
	BootstrapperBinaryUrl        string `json:"CHE_INFRA_KUBERNETES_BOOTSTRAPPER_BINARY__URL"`
	WorkspacesNamespace          string `json:"CHE_INFRA_OPENSHIFT_PROJECT"`
	PvcStrategy                  string `json:"CHE_INFRA_KUBERNETES_PVC_STRATEGY"`
	PvcClaimSize                 string `json:"CHE_INFRA_KUBERNETES_PVC_QUANTITY"`
	PvcJobsImage                 string `json:"CHE_INFRA_KUBERNETES_PVC_JOBS_IMAGE"`
	WorkspacePvcStorageClassName string `json:"CHE_INFRA_KUBERNETES_PVC_STORAGE__CLASS__NAME"`
	PreCreateSubPaths            string `json:"CHE_INFRA_KUBERNETES_PVC_PRECREATE__SUBPATHS"`
	TlsSupport                   string `json:"CHE_INFRA_OPENSHIFT_TLS__ENABLED"`
	K8STrustCerts                string `json:"CHE_INFRA_KUBERNETES_TRUST__CERTS"`
	DatabaseURL                  string `json:"CHE_JDBC_URL"`
	DbUserName                   string `json:"CHE_JDBC_USERNAME"`
	DbPassword                   string `json:"CHE_JDBC_PASSWORD"`
	CheLogLevel                  string `json:"CHE_LOG_LEVEL"`
	KeycloakURL                  string `json:"CHE_KEYCLOAK_AUTH__SERVER__URL"`
	KeycloakRealm                string `json:"CHE_KEYCLOAK_REALM"`
	KeycloakClientId             string `json:"CHE_KEYCLOAK_CLIENT__ID"`
	OpenShiftIdentityProvider    string `json:"CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"`
	JavaOpts                     string `json:"JAVA_OPTS"`
	WorkspaceJavaOpts            string `json:"CHE_WORKSPACE_JAVA__OPTIONS"`
	WorkspaceMavenOpts           string `json:"CHE_WORKSPACE_MAVEN__OPTIONS"`
	WorkspaceProxyJavaOpts       string `json:"CHE_WORKSPACE_HTTP__PROXY__JAVA__OPTIONS"`
	WorkspaceHttpProxy           string `json:"CHE_WORKSPACE_HTTP__PROXY"`
	WorkspaceHttpsProxy          string `json:"CHE_WORKSPACE_HTTPS__PROXY"`
	WorkspaceNoProxy             string `json:"CHE_WORKSPACE_NO__PROXY"`
	PluginRegistryUrl            string `json:"CHE_WORKSPACE_PLUGIN__REGISTRY__URL"`
	WebSocketEndpointMinor       string `json:"CHE_WEBSOCKET_ENDPOINT__MINOR"`
}

func GetCustomConfigMapData() (cheEnv map[string]string) {

	cheEnv = map[string]string{
		"CHE_PREDEFINED_STACKS_RELOAD__ON__START":               "true",
		"CHE_INFRA_KUBERNETES_SERVICE__ACCOUNT__NAME":           "che-workspace",
		"CHE_WORKSPACE_AUTO_START":                              "true",
		"CHE_INFRA_KUBERNETES_WORKSPACE__UNRECOVERABLE__EVENTS": "FailedMount,FailedScheduling,MountVolume.SetUp failed,Failed to pull image",
		"CHE_LIMITS_WORKSPACE_IDLE_TIMEOUT": "-1",
	}
	return cheEnv

}

// GetConfigMapData gets env values from CR spec and returns a map with key:value
// which is used in CheCluster ConfigMap to configure CheCluster master behavior
func GetConfigMapData(cr *orgv1.CheCluster) (cheEnv map[string]string) {
	cheHost := cr.Spec.Server.CheHost
	keycloakURL := cr.Spec.Auth.KeycloakURL
	isOpenShift, isOpenshift4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("Failed to get current infra: %s", err)
	}
	cheFlavor := util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor)
	chePostgresPassword := cr.Spec.Database.ChePostgresPassword
	infra := "kubernetes"
	if isOpenShift {
		infra = "openshift"
	}
	workspacesNamespace := cr.Namespace
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	openshiftOAuth := cr.Spec.Auth.OpenShiftOauth
	if openshiftOAuth && isOpenShift {
		workspacesNamespace = ""
		openShiftIdentityProviderId = "openshift-v3"
		if isOpenshift4 {
			openShiftIdentityProviderId = "openshift-v4"
		}
	}
	tlsSupport := cr.Spec.Server.TlsSupport
	protocol := "http"
	wsprotocol := "ws"
	if tlsSupport {
		protocol = "https"
		wsprotocol = "wss"
		tls = "true"
	}
	proxyJavaOpts := ""
	proxyUser := cr.Spec.Server.ProxyUser
	proxyPassword := cr.Spec.Server.ProxyPassword
	nonProxyHosts := cr.Spec.Server.NonProxyHosts
	if len(nonProxyHosts) < 1 && len(cr.Spec.Server.ProxyURL) > 1 {
		nonProxyHosts = os.Getenv("KUBERNETES_SERVICE_HOST")
	} else {
		nonProxyHosts = nonProxyHosts + "|" + os.Getenv("KUBERNETES_SERVICE_HOST")
	}
	if len(cr.Spec.Server.ProxyURL) > 1 {
		proxyJavaOpts = util.GenerateProxyJavaOpts(cr.Spec.Server.ProxyURL, cr.Spec.Server.ProxyPort, nonProxyHosts, proxyUser, proxyPassword)
	}
	cheWorkspaceHttpProxy := ""
	cheWorkspaceNoProxy := ""
	if len(cr.Spec.Server.ProxyURL) > 1 {
		cheWorkspaceHttpProxy, cheWorkspaceNoProxy = util.GenerateProxyEnvs(cr.Spec.Server.ProxyURL, cr.Spec.Server.ProxyPort, cr.Spec.Server.NonProxyHosts, proxyUser, proxyPassword)
	}

	ingressDomain := cr.Spec.K8SOnly.IngressDomain
	tlsSecretName := cr.Spec.K8SOnly.TlsSecretName
	securityContextFsGroup := util.GetValue(cr.Spec.K8SOnly.SecurityContextFsGroup, DefaultSecurityContextFsGroup)
	securityContextRunAsUser := util.GetValue(cr.Spec.K8SOnly.SecurityContextRunAsUser, DefaultSecurityContextRunAsUser)
	pvcStrategy := util.GetValue(cr.Spec.Storage.PvcStrategy, DefaultPvcStrategy)
	pvcClaimSize := util.GetValue(cr.Spec.Storage.PvcClaimSize, DefaultPvcClaimSize)
	workspacePvcStorageClassName := cr.Spec.Storage.WorkspacePVCStorageClassName
	
	defaultPVCJobsImage := DefaultPvcJobsUpstreamImage
	if cheFlavor == "codeready" {
		defaultPVCJobsImage = DefaultPvcJobsImage
	}
	pvcJobsImage := util.GetValue(cr.Spec.Storage.PvcJobsImage, defaultPVCJobsImage)
	preCreateSubPaths := "true"
	if !cr.Spec.Storage.PreCreateSubPaths {
		preCreateSubPaths = "false"
	}
	chePostgresHostName := util.GetValue(cr.Spec.Database.ChePostgresDBHostname, DefaultChePostgresHostName)
	chePostgresUser := util.GetValue(cr.Spec.Database.ChePostgresUser, DefaultChePostgresUser)
	chePostgresPort := util.GetValue(cr.Spec.Database.ChePostgresPort, DefaultChePostgresPort)
	chePostgresDb := util.GetValue(cr.Spec.Database.ChePostgresDb, DefaultChePostgresDb)
	keycloakRealm := util.GetValue(cr.Spec.Auth.KeycloakRealm, cheFlavor)
	keycloakClientId := util.GetValue(cr.Spec.Auth.KeycloakClientId, cheFlavor+"-public")
	ingressStrategy := util.GetValue(cr.Spec.K8SOnly.IngressStrategy, DefaultIngressStrategy)
	ingressClass := util.GetValue(cr.Spec.K8SOnly.IngressClass, DefaultIngressClass)
	pluginRegistryUrl := util.GetValue(cr.Spec.Server.PluginRegistryUrl, DefaultPluginRegistryUrl)
	cheLogLevel := util.GetValue(cr.Spec.Server.CheLogLevel, DefaultCheLogLevel)
	cheDebug := util.GetValue(cr.Spec.Server.CheDebug, DefaultCheDebug)

	data := &CheConfigMap{
		CheMultiUser:                 "true",
		CheHost:                      cheHost,
		ChePort:                      "8080",
		CheApi:                       protocol + "://" + cheHost + "/api",
		CheWebSocketEndpoint:         wsprotocol + "://" + cheHost + "/api/websocket",
		WebSocketEndpointMinor:       wsprotocol + "://" + cheHost + "/api/websocket-minor",
		CheDebugServer:               cheDebug,
		CheInfrastructureActive:      infra,
		BootstrapperBinaryUrl:        protocol + "://" + cheHost + "/agent-binaries/linux_amd64/bootstrapper/bootstrapper",
		WorkspacesNamespace:          workspacesNamespace,
		PvcStrategy:                  pvcStrategy,
		PvcClaimSize:                 pvcClaimSize,
		WorkspacePvcStorageClassName: workspacePvcStorageClassName,
		PvcJobsImage:                 pvcJobsImage,
		PreCreateSubPaths:            preCreateSubPaths,
		TlsSupport:                   tls,
		K8STrustCerts:                tls,
		DatabaseURL:                  "jdbc:postgresql://" + chePostgresHostName + ":" + chePostgresPort + "/" + chePostgresDb,
		DbUserName:                   chePostgresUser,
		DbPassword:                   chePostgresPassword,
		CheLogLevel:                  cheLogLevel,
		KeycloakURL:                  keycloakURL + "/auth",
		KeycloakRealm:                keycloakRealm,
		KeycloakClientId:             keycloakClientId,
		OpenShiftIdentityProvider:    openShiftIdentityProviderId,
		JavaOpts:                     DefaultJavaOpts + " " + proxyJavaOpts,
		WorkspaceJavaOpts:            DefaultWorkspaceJavaOpts + " " + proxyJavaOpts,
		WorkspaceMavenOpts:           DefaultWorkspaceJavaOpts + " " + proxyJavaOpts,
		WorkspaceProxyJavaOpts:       proxyJavaOpts,
		WorkspaceHttpProxy:           cheWorkspaceHttpProxy,
		WorkspaceHttpsProxy:          cheWorkspaceHttpProxy,
		WorkspaceNoProxy:             cheWorkspaceNoProxy,
		PluginRegistryUrl:            pluginRegistryUrl,
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
		"CHE_INFRA_KUBERNETES_NAMESPACE":                           workspacesNamespace,
		"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":                      ingressDomain,
		"CHE_INFRA_KUBERNETES_SERVER__STRATEGY":                    ingressStrategy,
		"CHE_INFRA_KUBERNETES_TLS__SECRET":                         tlsSecretName,
		"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON":           "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/\",\"nginx.ingress.kubernetes.io/ssl-redirect\": " + tls + ",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
	}
	if !isOpenShift {
		addMap(cheEnv, k8sCheEnv)
	}
	return cheEnv
}

func NewCheConfigMap(cr *orgv1.CheCluster, cheEnv map[string]string) *corev1.ConfigMap {
	labels := GetLabels(cr, util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor))
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che",
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: cheEnv,
	}
}
