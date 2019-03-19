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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewKeycloakDeployment(cr *orgv1.CheCluster, keycloakPostgresPassword string, keycloakAdminPassword string, cheFlavor string) *appsv1.Deployment {
	optionalEnv := true
	keycloakName := "keycloak"
	labels := GetLabels(cr, keycloakName)
	keycloakImage := util.GetValue(cr.Spec.Auth.KeycloakImage, DefaultKeycloakImage)
	trustpass := util.GeneratePasswd(12)
	jbossDir := "/opt/eap"
	if cheFlavor == "che" {
		// writable dir in the upstream Keycloak image
		jbossDir = "/scripts"
	}
	// add crt to Java trust store so that Keycloak can connect to k8s API
	addCertToTrustStoreCommand := "if [ ! -z \"${CHE_SELF__SIGNED__CERT}\" ]; then echo \"${CHE_SELF__SIGNED__CERT}\" > " + jbossDir + "/openshift.crt && " +
		"keytool -importcert -alias HOSTDOMAIN0" +
		" -keystore " + jbossDir +"/openshift.jks" +
		" -file " + jbossDir + "/openshift.crt -storepass " + trustpass + " -noprompt; fi" +
		" && keytool -importcert -alias HOSTDOMAIN" +
		" -keystore " + jbossDir +"/openshift.jks" +
		" -file /var/run/secrets/kubernetes.io/serviceaccount/ca.crt -storepass " + trustpass + " -noprompt" +
		" && keytool -importkeystore -srckeystore $JAVA_HOME/jre/lib/security/cacerts" +
		" -destkeystore " + jbossDir + "/openshift.jks" +
		" -srcstorepass changeit -deststorepass " + trustpass
	startCommand := "sed -i 's/WILDCARD/ANY/g' /opt/eap/bin/launch/keycloak-spi.sh && /opt/eap/bin/openshift-launch.sh -b 0.0.0.0"
	// upstream Keycloak has a bit different mechanism of adding jks
	changeConfigCommand := "echo -e \"embed-server --server-config=standalone.xml --std-out=echo \n" +
		"/subsystem=keycloak-server/spi=truststore/:add \n" +
		"/subsystem=keycloak-server/spi=truststore/provider=file/:add(properties={file => " +
		"\"" + jbossDir + "/openshift.jks\", password => \"" + trustpass + "\", disabled => \"false\" },enabled=true) \n" +
		"stop-embedded-server\" > /scripts/add_openshift_certificate.cli && " +
		"/opt/jboss/keycloak/bin/jboss-cli.sh --file=/scripts/add_openshift_certificate.cli"
	keycloakAdminUserName := util.GetValue(cr.Spec.Auth.KeycloakAdminUserName, DefaultKeycloakAdminUserName)
	keycloakEnv := []corev1.EnvVar{
		{
			Name:  "PROXY_ADDRESS_FORWARDING",
			Value: "true",
		},
		{
			Name:  "KEYCLOAK_USER",
			Value: keycloakAdminUserName,
		},
		{
			Name:  "KEYCLOAK_PASSWORD",
			Value: keycloakAdminPassword,
		},
		{
			Name:  "DB_VENDOR",
			Value: "POSTGRES",
		},
		{
			Name:  "POSTGRES_PORT_5432_TCP_ADDR",
			Value: "postgres",
		},
		{
			Name:  "POSTGRES_PORT_5432_TCP_PORT",
			Value: "5432",
		},
		{
			Name:  "POSTGRES_DATABASE",
			Value: "keycloak",
		},
		{
			Name:  "POSTGRES_USER",
			Value: "keycloak",
		},
		{
			Name:  "POSTGRES_PASSWORD",
			Value: keycloakPostgresPassword,
		},
		{
			Name: "CHE_SELF__SIGNED__CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "ca.crt",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "self-signed-certificate",
					},
					Optional: &optionalEnv,
				},
			},
		},
	}
	if cheFlavor == "codeready" {
		keycloakEnv = []corev1.EnvVar{
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
			{
				Name:  "DB_PASSWORD",
				Value: keycloakPostgresPassword,
			},
			{
				Name:  "SSO_ADMIN_USERNAME",
				Value: keycloakAdminUserName,
			},
			{
				Name:  "SSO_ADMIN_PASSWORD",
				Value: keycloakAdminPassword,
			},
			{
				Name:  "DB_VENDOR",
				Value: "POSTGRES",
			},
			{
				Name:  "SSO_TRUSTSTORE",
				Value: "openshift.jks",
			},
			{
				Name:  "SSO_TRUSTSTORE_DIR",
				Value: jbossDir,
			},
			{
				Name:  "SSO_TRUSTSTORE_PASSWORD",
				Value: trustpass,
			},
			{
				Name: "CHE_SELF__SIGNED__CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "ca.crt",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "self-signed-certificate",
						},
						Optional: &optionalEnv,
					},
				},
			},
		}
	}
	command := addCertToTrustStoreCommand + " && " + changeConfigCommand + " && /opt/jboss/docker-entrypoint.sh -b 0.0.0.0"
	if cheFlavor == "codeready" {
		command = addCertToTrustStoreCommand + " && " + startCommand
	}
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      keycloakName,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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
