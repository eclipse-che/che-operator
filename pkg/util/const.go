package util

const (
	// Docker image for Che server. Defaults to eclipse/che-server:latest
	CheImageRepo = "CHE_IMAGE_REPO"
	// Docker image for Che server. Defaults to eclipse/che-server:latest
	CheImageTag = "CHE_IMAGE_TAG"
	// support of https routes
	TlsSupport = "TLS_SUPPORT"
	// PVC strategy for Che workspaces
	PvcStrategy = "PVC_STRATEGY"
	// PVC claim size
	PvcClaimSize = "PVC_STRATEGY"
	// base64 -w 0 of self signed certificate
	SelfSignedCert = "SELF_SIGNED_CERT"
	// enable Login with OpenShift in Code ready Workspaces
	OpenShiftOauth = "OPENSHIFT_OAUTH"
	// OpenShift API endpoint URL. Required only when OPENSHHIFT_OAUTH is true
	OpenShiftApiUrl = "OPENSHIFT_API_URL"
	// UDeploy Postgres or use existing DB
	ExternalDb = "EXTERNAL_DB"
	// Provide external DB hostname
	ExternalDbHostname = "DB_HOSTNAME"
	// Provide external DB port
	ExternalDbPort = "DB_PORT"
	// Provide external DB database
	ExternalDbDatabase = "DB_DATABASE"
	// Provide external DB username
	ExternalDbUsername = "DB_USERNAME"
	// Provide external DB password
	ExternalDbPassword = "DB_PASSWORD"
	// Deploy Keycloak or use existing Keycloak auth server
	ExternalKeycloak = "EXTERNAL_KEYCLOAK"
	// External Keycloak/Red Hat SSO
	ExternalKeycloakUrl = "KEYCLOAK_URL"
	// Keycloak admin name
	ExternalKeycloakAdminUserName = "KEYCLOAK_ADMIN_USERNAME"
	// Keycloak admin password
	ExternalKeycloakAdminPassword = "KEYCLOAK_ADMIN_PASSWORD"
	// External Red Hat SSO realm
	ExternalKeycloakRealm = "KEYCLOAK_REALM"
	// External Red Hat SSO client ID
	ExternalKeycloakClientId = "KEYCLOAK_CLIENT_ID"
	//ingress domain for k8s
	IngressDomain = "INGRESS_DOMAIN"
	// ingress class
	IngressClass = "INGRESS_CLASS"
	// ingress strategy
	Strategy      = "INGRESS_STRATEGY"
	TlsSecretName = "TLS_SECRET_NAME"
	// fake DNS if you need it in deployments
	HostAliasIP       = "HOST_ALIAS_IP"
	HostAliasHostname = "HOST_ALIAS_HOSTNAME"
)
