package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
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
	addCertToTrustStoreCommand = "echo \"${SELF_SIGNED_CERTIFICATE}\" > /opt/eap/bin/openshift.crt" +
		" && keytool -importcert -alias HOSTDOMAIN" +
		" -keystore /opt/eap/bin/openshift.jks" +
		" -file /opt/eap/bin/openshift.crt -storepass " + trustpass + " -noprompt" +
		" && keytool -importkeystore -srckeystore $JAVA_HOME/jre/lib/security/cacerts" +
		" -destkeystore /opt/eap/bin/openshift.jks" +
		" -srcstorepass changeit -deststorepass " + trustpass

	trustStoreCommandArg = " --truststore /opt/eap/bin/openshift.jks --trustpass " + trustpass + " "
	startCommand         = "/opt/eap/bin/openshift-launch.sh"
)

func newKeycloakDeployment() *appsv1.Deployment {
	optionalEnv := true
	var command string
	selfSignedCert := util.GetEnv(util.SelfSignedCert, "")
	ssoTrustStoreEnv := corev1.EnvVar{Name: "SSO_TRUSTSTORE", Value: "openshift.jks"}
	ssoTrustStoreDir := corev1.EnvVar{Name: "SSO_TRUSTSTORE_DIR", Value: "/opt/eap/bin"}
	ssoTrustStorePassword := corev1.EnvVar{Name: "SSO_TRUSTSTORE_PASSWORD", Value: trustpass,}

	keycloakEnv := []corev1.EnvVar{
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
		{
			Name: "SELF_SIGNED_CERTIFICATE",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "ca.crt",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "self-signed-cert",
					},
					Optional: &optionalEnv,
				},
			},
		},
	}

	if len(selfSignedCert) > 0 {
		command = addCertToTrustStoreCommand + " && " + startCommand
		keycloakEnv = append(keycloakEnv, ssoTrustStoreDir, ssoTrustStorePassword, ssoTrustStoreEnv)
	} else {
		command = startCommand
	}

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
				Type: appsv1.DeploymentStrategyType("Recreate"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: keycloakLabels,
				},
				Spec: corev1.PodSpec{
					// testing https on k8s
					HostAliases: hostAliases,
					Containers: []corev1.Container{
						{
							Name:  keycloakName,
							Image: keycloakImage,
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
	deployment := newKeycloakDeployment()
	if err := sdk.Create(deployment); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+keycloakName+" deployment : %v", err)
		return nil, err
	}

	// wait until deployment is scaled to 1 replica to proceed with other deployments
	util.WaitForSuccessfulDeployment(deployment, "Keycloak", 40)
	return deployment, nil
}

func newKeycloakJob( keycloakURL string, cheHost string) *batchv1.Job {
	if tlsSupport {
		protocol = "https"
	}
	optionalEnv := true
	labels := map[string]string{"app": "kc-job"}
	var command string
	var openshiftOAuth = util.GetEnvBool(util.OpenShiftOauth, false)
	openShiftApiUrl := util.GetEnv(util.OpenShiftApiUrl, "")
	var backoffLimit int64 = 40

	// read entrypoint file, convert it to string and replace envs with real values
	// it's better to have entrypoint command in a readable format rather than as a
	// long string with escape characters
	file, err := ioutil.ReadFile("/tmp/keycloak_provision") //
	if err != nil {
		logrus.Errorf("Failed to find keycloak entrypoint file", err)
	}
	str := string(file)
	r := strings.NewReplacer("$keycloakURL", keycloakURL,
									"$keycloakAdminUserName", keycloakAdminUserName,
									"$keycloakAdminPassword", keycloakAdminPassword,
									"$keycloakRealm", keycloakRealm,
									"$keycloakClientId", keycloakClientId,
									"$protocol", protocol,
									"$cheHost", cheHost,
									"$trustStoreCommandArg", trustStoreCommandArg)
	createRealmClientUserCommand := r.Replace(str)


	createOpenShiftIdentityProviderCommand :=
		"/opt/eap/bin/kcadm.sh create identity-provider/instances -r " + keycloakRealm +
			" -s alias=openshift-v3 -s providerId=openshift-v3 -s enabled=true -s storeToken=true" +
			" -s addReadTokenRoleOnCreate=true -s config.useJwksUrl=true" +
			" -s config.clientId=openshift-identity-provider -s config.clientSecret=" + oauthSecret +
			" -s config.baseUrl=" + openShiftApiUrl +
			" -s config.defaultScope=user:full" + trustStoreCommandArg

	command = createRealmClientUserCommand
	if len(selfSignedCert) > 0 {
		command = addCertToTrustStoreCommand + " && " + createRealmClientUserCommand
	}
	if len(selfSignedCert) > 0 && openshiftOAuth {
		command = addCertToTrustStoreCommand + " && " + createRealmClientUserCommand + " && " + createOpenShiftIdentityProviderCommand
	}
	if openshiftOAuth {
		command = createRealmClientUserCommand + " && " + createOpenShiftIdentityProviderCommand
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kc-job",
			Namespace: namespace,

			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kc-job",
					Namespace: namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					HostAliases:   hostAliases,
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "kc-service-pod",
							Image:   "registry.access.redhat.com/redhat-sso-7/sso72-openshift:1.2-8",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{"/bin/bash"},
							Args: []string{
								"-c",
								command,
							},
							Env: []corev1.EnvVar{
								{
									Name: "SELF_SIGNED_CERTIFICATE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "ca.crt",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "self-signed-cert",
											},
											Optional: &optionalEnv,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// CreateKeycloakJob creates a Job that starts a pod with kcadm.sh utility to provision Keycloak resources such as realm, client, identity provider etc
func CreateKeycloakJob( keycloakURL string, cheHost string) {
	job := newKeycloakJob( keycloakURL, cheHost)
	if err := sdk.Create(job); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create Keycloak service pod : %v", err)
	}
	util.WaitForSuccessfulJobExecution(job, "Keycloak", 10)

}
