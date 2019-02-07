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
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
)

var (
	keycloakName               = "keycloak"
	keycloakImage              = "registry.access.redhat.com/redhat-sso-7/sso72-openshift:1.2-8"
	trustpass                  = util.GeneratePasswd(12)
	addCertToTrustStoreCommand = "keytool -importcert -alias HOSTDOMAIN" +
			" -keystore /opt/eap/bin/openshift.jks" +
			" -file /var/run/secrets/kubernetes.io/serviceaccount/ca.crt -storepass " + trustpass + " -noprompt" +
			" && keytool -importkeystore -srckeystore $JAVA_HOME/jre/lib/security/cacerts" +
			" -destkeystore /opt/eap/bin/openshift.jks" +
			" -srcstorepass changeit -deststorepass " + trustpass

	trustStoreCommandArg = " --truststore /opt/eap/bin/openshift.jks --trustpass " + trustpass + " "
	startCommand         = "sed -i 's/WILDCARD/ANY/g' /opt/eap/bin/launch/keycloak-spi.sh && /opt/eap/bin/openshift-launch.sh -b 0.0.0.0"
)

func newKeycloakDeployment() *appsv1.Deployment {

	keycloakEnv := []corev1.EnvVar{
		{
			Name:  "SSO_TRUSTSTORE",
			Value: "openshift.jks",
		},
		{
			Name:  "SSO_TRUSTSTORE_DIR",
			Value: "/opt/eap/bin"},
		{
			Name:  "SSO_TRUSTSTORE_PASSWORD",
			Value: trustpass,
		},
		{
			Name:  "PROXY_ADDRESS_FORWARDING",
			Value: "true",
		},
		{
			Name:  "DB_SERVICE_PREFIX_MAPPING",
			Value: "keycloak-postgresql=DB",
		},
		{
			Name:  "KEYCLOAK_POSTGRESQL_SERVICE_HOST",
			Value: "postgres",
		},
		{
			Name:  "KEYCLOAK_POSTGRESQL_SERVICE_PORT",
			Value: "5432",
		},
		{
			Name:  "DB_DATABASE",
			Value: keycloakName,
		},
		{
			Name:  "DB_USERNAME",
			Value: keycloakName,
		},
		// todo Create a secret for it?
		{
			Name:  "DB_PASSWORD",
			Value: keycloakPostgresPassword,
		},
		{
			Name:  "SSO_ADMIN_USERNAME",
			Value: keycloakAdminUserName,
		},
		// todo Create a secret for it?
		{
			Name:  "SSO_ADMIN_PASSWORD",
			Value: keycloakAdminPassword,
		},
		{
			Name:  "DB_VENDOR",
			Value: "POSTGRES",
		},
	}

	command := addCertToTrustStoreCommand + " && " + startCommand
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      keycloakName,
			Namespace: namespace,
			Labels:    keycloakLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: keycloakLabels},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: keycloakLabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            keycloakName,
							Image:           keycloakImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/bin/sh",
							},
							Args: []string{
								"-c",
								command,
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          keycloakName,
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2Gi"),
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "auth/js/keycloak.js",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 25,
								FailureThreshold:    10,
								TimeoutSeconds:      5,
							},
							Env: keycloakEnv,
						},
					},
				},
			},
		},
	}
}

// CreateKeycloakDeployment creates a deployment that starts a container with Keycloak
func CreateKeycloakDeployment() (*appsv1.Deployment, error) {
	k8s := GetK8SConfig()
	deployment := newKeycloakDeployment()
	if err := sdk.Create(deployment); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+keycloakName+" deployment : %v", err)
		return nil, err
	}
	// wait until deployment is scaled to 1 replica to proceed with other deployments
	k8s.GetDeploymentStatus(deployment)
	return deployment, nil
}

func GetKeycloakProvisionCommand(keycloakURL string, cheHost string) (command string) {
	openShiftApiUrl := util.GetEnv("CHE_OPENSHIFT_API_URL", "")
	requiredActions := ""
	if updateAdminPassword {
		requiredActions = "\"UPDATE_PASSWORD\""
	}
	file, err := ioutil.ReadFile("/tmp/keycloak_provision") //
	if err != nil {
		logrus.Errorf("Failed to find keycloak entrypoint file", err)
	}
	keycloakTheme := "keycloak"
	if cheFlavor == "codeready" {
		keycloakTheme = "rh-sso"

	}
	str := string(file)
	r := strings.NewReplacer("$keycloakURL", keycloakURL,
		"$keycloakAdminUserName", keycloakAdminUserName,
		"$keycloakAdminPassword", keycloakAdminPassword,
		"$keycloakRealm", keycloakRealm,
		"$keycloakClientId", keycloakClientId,
		"$keycloakTheme", keycloakTheme,
		"$protocol", protocol,
		"$cheHost", cheHost,
		"$trustStoreCommandArg", trustStoreCommandArg,
		"$requiredActions", requiredActions)
	createRealmClientUserCommand := r.Replace(str)

	createOpenShiftIdentityProviderCommand :=
		"/opt/eap/bin/kcadm.sh create identity-provider/instances -r " + keycloakRealm +
			" -s alias=openshift-v3 -s providerId=openshift-v3 -s enabled=true -s storeToken=true" +
			" -s addReadTokenRoleOnCreate=true -s config.useJwksUrl=true" +
			" -s config.clientId=" + oAuthClientName + " -s config.clientSecret=" + oauthSecret +
			" -s config.baseUrl=" + openShiftApiUrl +
			" -s config.defaultScope=user:full"

	command = createRealmClientUserCommand
	if openshiftOAuth {
		command = createRealmClientUserCommand + " && " + createOpenShiftIdentityProviderCommand
	}
	return command
}
