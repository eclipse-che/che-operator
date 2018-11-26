package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newCheConfigMap(cheHost string, keycloakURL string) *corev1.ConfigMap {
	cheLabels := map[string]string{"app": "che"}
	openshiftOAuth := util.GetEnvBool(util.OpenShiftOauth, false)
	ingressDomain := util.GetEnv(util.IngressDomain, "")
	strategy := util.GetEnv(util.Strategy, "default-host")
	workspacesNamespace := namespace
	tls := "false"
	openShiftIdentityProviderId := "NULL"
	if openshiftOAuth {
		workspacesNamespace = ""
		openShiftIdentityProviderId = "openshift-v3"
	}
	if tlsSupport {
		protocol = "https"
		wsprotocol = "wss"
		tls = "true"
	}

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che",
			Namespace: namespace,
			Labels:    cheLabels,
		},
		Data: map[string]string{

			"CHE_MULTIUSER":          "true",
			"CHE_HOST":               cheHost,
			"CHE_PORT":               "8080",
			"CHE_API":                protocol + "://" + cheHost + "/api",
			"CHE_WEBSOCKET_ENDPOINT": wsprotocol + "://" + cheHost + "/api/websocket",
			// todo Make configurable
			"CHE_DEBUG_SERVER":                              "false",
			"CHE_INFRASTRUCTURE_ACTIVE":                     infra,
			"CHE_INFRA_KUBERNETES_BOOTSTRAPPER_BINARY__URL": protocol + "://" + cheHost + "/agent-binaries/linux_amd64/bootstrapper/bootstrapper",
			"CHE_INFRA_OPENSHIFT_PROJECT":                   workspacesNamespace,
			// todo Make configurable
			"CHE_INFRA_KUBERNETES_PVC_STRATEGY": pvcStrategy,
			"CHE_INFRA_KUBERNETES_PVC_QUANTITY": pvcClaimSize,
			"CHE_INFRA_OPENSHIFT_TLS__ENABLED":  tls,
			"CHE_INFRA_KUBERNETES_TRUST__CERTS": tls,
			"CHE_JDBC_URL":                      "jdbc:postgresql://" + postgresHostName + ":" + postgresPort + "/" + chePostgresDb,
			"CHE_JDBC_USERNAME":                 chePostgresUser,
			// todo Create a secret for it?
			"CHE_JDBC_PASSWORD":                             chePostgresPassword,
			"CHE_LOG_LEVEL":                                 "INFO",
			"CHE_KEYCLOAK_AUTH__SERVER__URL":                keycloakURL + "/auth",
			"CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER": openShiftIdentityProviderId,
			"CHE_PREDEFINED_STACKS_RELOAD__ON__START":       "true",
			"CHE_INFRA_KUBERNETES_SERVICE__ACCOUNT__NAME":   "che-workspace",
			"JAVA_OPTS":                                             "-XX:MaxRAMFraction=2 -XX:+UseParallelGC -XX:MinHeapFreeRatio=10 -XX:MaxHeapFreeRatio=20 -XX:GCTimeRatio=4 -XX:AdaptiveSizePolicyWeight=90 -XX:+UnlockExperimentalVMOptions -XX:+UseCGroupMemoryLimitForHeap -Dsun.zip.disableMemoryMapping=true -Xms20m ",
			"CHE_WORKSPACE_AUTO_START":                              "true",
			"CHE_KEYCLOAK_REALM":                                    keycloakRealm,
			"CHE_KEYCLOAK_CLIENT__ID":                               keycloakClientId,
			"CHE_INFRA_KUBERNETES_WORKSPACE__UNRECOVERABLE__EVENTS": "FailedMount,FailedScheduling,MountVolume.SetUp failed,Failed to pull image",
			// todo add global plugin registry
			// todo support k8s tls
			// k8s specific envs
			"CHE_INFRA_KUBERNETES_NAMESPACE":                 workspacesNamespace,
			"CHE_INFRA_KUBERNETES_INGRESS_DOMAIN":            ingressDomain,
			"CHE_INFRA_KUBERNETES_SERVER__STRATEGY":          strategy,
			"CHE_INFRA_KUBERNETES_INGRESS_ANNOTATIONS__JSON": "{\"kubernetes.io/ingress.class\": " + ingressClass + ", \"nginx.ingress.kubernetes.io/rewrite-target\": \"/\",\"nginx.ingress.kubernetes.io/ssl-redirect\": " + tls + ",\"nginx.ingress.kubernetes.io/proxy-connect-timeout\": \"3600\",\"nginx.ingress.kubernetes.io/proxy-read-timeout\": \"3600\"}",
		},
	}
}

// CreateCheConfigMap creates a ConfigMaps that holds all Che configuration
func CreateCheConfigMap(cheHost string, keycloakURL string) *corev1.ConfigMap {
	cm := newCheConfigMap(cheHost, keycloakURL)
	if err := sdk.Create(cm); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create Che ConfigMap : %v", err)
		return nil
	}
	return cm
}
