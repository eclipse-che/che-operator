package operator

import ("github.com/eclipse/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

var (
	// general config
	namespace = util.GetNamespace()
	infra             = util.GetInfra()
	protocol          = "http"
	wsprotocol        = "ws"
	cheHost           string
	tlsSupport        = util.GetEnvBool(util.TlsSupport, false)
	pvcStrategy = util.GetEnv(util.PvcStrategy, "common")
	pvcClaimSize = util.GetEnv(util.PvcClaimSize,"1Gi")
	selfSignedCert    = util.GetEnv(util.SelfSignedCert, "")
	openshiftOAuth    = util.GetEnvBool(util.OpenShiftOauth, false)
	oauthSecret       = util.GeneratePasswd(12)
	hostAliasIP       = util.GetEnv(util.HostAliasIP, "10.10.10.10")
	hostAliasHostname = util.GetEnv(util.HostAliasHostname, "example.com")
	hostAliases = []corev1.HostAlias{
		{
			IP: hostAliasIP,
			Hostnames: []string{
				hostAliasHostname,
			},
		},
	}

	// k8s specific config

	ingressDomain     = util.GetEnv(util.IngressDomain, "192.168.42.114")
	strategy          = util.GetEnv(util.Strategy, "multi-host")
	ingressClass = util.GetEnv(util.IngressClass, "nginx")
	tlsSecretName     = util.GetEnv(util.TlsSecretName, "")

	// postgres config
	externalDb            = util.GetEnvBool(util.ExternalDb, false)
	postgresHostName      = util.GetEnv(util.ExternalDbHostname, "postgres")
	postgresPort          = util.GetEnv(util.ExternalDbPort, "5432")
	chePostgresDb         = util.GetEnv(util.ExternalDbDatabase, "dbche")
	chePostgresUser       = util.GetEnv(util.ExternalDbUsername, "pgche")
	chePostgresPassword   = util.GetEnv(util.ExternalDbPassword, util.GeneratePasswd(12))
	postgresAdminPassword = util.GeneratePasswd(12)

	// Keycloak config
	externalKeycloak         = util.GetEnvBool(util.ExternalKeycloak, false)
	keycloakURL              = util.GetEnv(util.ExternalKeycloakUrl, "")
	keycloakAdminUserName    = util.GetEnv(util.ExternalKeycloakAdminUserName, "admin")
	keycloakAdminPassword    = util.GetEnv(util.ExternalKeycloakAdminPassword, util.GeneratePasswd(12))
	keycloakPostgresPassword = util.GeneratePasswd(10)
	keycloakRealm            = util.GetEnv(util.ExternalKeycloakRealm, "codeready")
	keycloakClientId         = util.GetEnv(util.ExternalKeycloakClientId, "codeready-public")

	cheImageRepo = util.GetEnv(util.CheImageRepo, "eclipse/che-server")
	cheImageTag  = util.GetEnv(util.CheImageTag, "latest")

	postgresLabels = map[string]string{"app": "postgres"}
	keycloakLabels = map[string]string{"app": "keycloak"}
	cheLabels      = map[string]string{"app": "che"}
)
