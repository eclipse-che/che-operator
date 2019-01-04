//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"strings"
)

var (
	// general config
	cheFlavor           = util.GetEnv("CHE_FLAVOR", "che")
	namespace           = util.GetNamespace()
	protocol            = "http"
	wsprotocol          = "ws"
	cheHost             string
	tlsSupport          = util.GetEnvBool("CHE_TLS_SUPPORT", false)
	pvcStrategy         = util.GetEnv("CHE_INFRA_KUBERNETES_PVC_STRATEGY", "common")
	pvcClaimSize        = util.GetEnv("CHE_INFRA_KUBERNETES_PVC_QUANTITY", "1Gi")
	selfSignedCert      = util.GetEnv("CHE_SELF__SIGNED__CERT", "")
	openshiftOAuth      = util.GetEnvBool("CHE_OPENSHIFT_OAUTH", false)
	oauthSecret         = util.GeneratePasswd(12)
	oAuthClientName     = "openshift-identity-provider-" + strings.ToLower(util.GeneratePasswd(4))
	updateAdminPassword = util.GetEnvBool("CHE_UPDATE_CHE_ADMIN_PASSWORD", true)

	// proxy config

	cheWsmasterProxyJavaOptions  = util.GetEnv("CHE_WORKSPACE_MASTER_PROXY_JAVA_OPTS", "")
	cheWorkspaceProxyJavaOptions = util.GetEnv("CHE_WORKSPACE_PROXY_JAVA_OPTS", "")
	cheWorkspaceHttpProxy        = util.GetEnv("CHE_WORKSPACE_HTTP__PROXY", "")
	cheWorkspaceHttpsProxy       = util.GetEnv("CHE_WORKSPACE_HTTPS__PROXY", "")
	cheWorkspaceNoProxy          = util.GetEnv("CHE_WORKSPACE_NO__PROXY", "")

	// plugin registry url
	pluginRegistryUrl = util.GetEnv("CHE_WORKSPACE_PLUGIN__REGISTRY__URL", "https://che-plugin-registry.openshift.io")

	// k8s specific config

	ingressDomain = util.GetEnv("CHE_INFRA_KUBERNETES_INGRESS_DOMAIN", "192.168.42.114")
	strategy      = util.GetEnv("CHE_INFRA_KUBERNETES_SERVER__STRATEGY", "multi-host")
	ingressClass  = util.GetEnv("INGRESS_CLASS", "nginx")
	tlsSecretName = util.GetEnv("CHE_INFRA_KUBERNETES_TLS__SECRET", "")
	// postgres config
	externalDb            = util.GetEnvBool("CHE_EXTERNAL_DB", false)
	postgresHostName      = util.GetEnv("CHE_DB_HOSTNAME", "postgres")
	postgresPort          = util.GetEnv("CHE_DB_PORT", "5432")
	chePostgresDb         = util.GetEnv("CHE_DB_DATABASE", "dbche")
	chePostgresUser       = util.GetEnv("CHE_JDBC_USERNAME", "pgche")
	chePostgresPassword   = util.GetEnv("CHE_JDBC_PASSWORD", util.GeneratePasswd(12))
	postgresAdminPassword = util.GeneratePasswd(12)

	// Keycloak config
	externalKeycloak         = util.GetEnvBool("CHE_EXTERNAL_KEYCLOAK", false)
	keycloakURL              = util.GetEnv("CHE_KEYCLOAK_AUTH__SERVER__URL", "")
	keycloakAdminUserName    = util.GetEnv("CHE_KEYCLOAK_ADMIN_USERNAME", "admin")
	keycloakAdminPassword    = util.GetEnv("CHE_KEYCLOAK_ADMIN_PASSWORD", util.GeneratePasswd(12))
	keycloakPostgresPassword = util.GeneratePasswd(10)
	keycloakRealm            = util.GetEnv("CHE_KEYCLOAK_REALM", cheFlavor)
	keycloakClientId         = util.GetEnv("CHE_KEYCLOAK_CLIENT__ID", cheFlavor+"-public")

	cheImage = util.GetEnv("CHE_IMAGE", "eclipse/che-server:latest")

	postgresLabels = map[string]string{"app": "postgres"}
	keycloakLabels = map[string]string{"app": "keycloak"}
	cheLabels      = map[string]string{"app": "che"}
)
