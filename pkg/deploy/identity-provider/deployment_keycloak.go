//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package identity_provider

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/postgres"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	selectSslRequiredCommand = "OUT=$(psql keycloak -tAc \"SELECT 1 FROM REALM WHERE id = 'master'\"); " +
		"if [ $OUT -eq 1 ]; then psql keycloak -tAc \"SELECT ssl_required FROM REALM WHERE id = 'master'\"; fi"
	updateSslRequiredCommand = "psql keycloak -c \"update REALM set ssl_required='NONE' where id = 'master'\""
)

var (
	trustpass              = util.GeneratePasswd(12)
	keycloakCustomDiffOpts = cmp.Options{
		deploy.DefaultDeploymentDiffOpts,
		cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
			return x.Annotations["che.self-signed-certificate.version"] == y.Annotations["che.self-signed-certificate.version"] &&
				x.Annotations["che.openshift-api-crt.version"] == y.Annotations["che.openshift-api-crt.version"] &&
				x.Annotations["che.keycloak-ssl-required-updated"] == y.Annotations["che.keycloak-ssl-required-updated"]
		}),
	}
)

func SyncKeycloakDeploymentToCluster(deployContext *deploy.DeployContext) (bool, error) {
	actual := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(deployContext, deploy.IdentityProviderName, actual)
	if err != nil {
		return false, err
	}

	if !exists {
		actual = nil
	}

	specDeployment, err := GetSpecKeycloakDeployment(deployContext, actual)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(deployContext, specDeployment, deploy.DefaultDeploymentDiffOpts)
}

func GetSpecKeycloakDeployment(
	deployContext *deploy.DeployContext,
	clusterDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	optionalEnv := true
	labels, labelSelector := deploy.GetLabelsAndSelector(deployContext.CheCluster, deploy.IdentityProviderName)
	cheFlavor := deploy.DefaultCheFlavor(deployContext.CheCluster)
	keycloakImage := util.GetValue(deployContext.CheCluster.Spec.Auth.IdentityProviderImage, deploy.DefaultKeycloakImage(deployContext.CheCluster))
	pullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Auth.IdentityProviderImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(keycloakImage)))
	jbossDir := "/opt/eap"
	if cheFlavor == "che" {
		// writable dir in the upstream Keycloak image
		jbossDir = "/scripts"
	}
	jbossCli := "/opt/jboss/keycloak/bin/jboss-cli.sh"
	if cheFlavor == "codeready" {
		jbossCli = "/opt/eap/bin/jboss-cli.sh"
	}

	if clusterDeployment != nil {
		// To be compatible with prev deployments when "TRUSTPASS" env was used
		clusterContainer := &clusterDeployment.Spec.Template.Spec.Containers[0]
		env := util.FindEnv(clusterContainer.Env, "TRUSTPASS")
		if env != nil {
			trustpass = env.Value
		} else {
			env := util.FindEnv(clusterContainer.Env, "SSO_TRUSTSTORE_PASSWORD")
			if env != nil {
				trustpass = env.Value
			}
		}
	}

	cmResourceVersions := tls.GetAdditionalCACertsConfigMapVersion(deployContext)
	terminationGracePeriodSeconds := int64(30)
	cheCertSecretVersion := getSecretResourceVersion("self-signed-certificate", deployContext.CheCluster.Namespace, deployContext.ClusterAPI)
	openshiftApiCertSecretVersion := getSecretResourceVersion("openshift-api-crt", deployContext.CheCluster.Namespace, deployContext.ClusterAPI)

	// holds bash functions that should be available when run init commands in shell
	bashFunctions := ""

	// add various certificates to Java trust store so that Keycloak can connect to OpenShift API
	// certificate that OpenShift router uses (for 4.0 only)
	addRouterCrt := "if [ ! -z \"${CHE_SELF__SIGNED__CERT}\" ]; then echo \"${CHE_SELF__SIGNED__CERT}\" > " + jbossDir + "/che.crt && " +
		"keytool -importcert -alias ROUTERCRT" +
		" -keystore " + jbossDir + "/openshift.jks" +
		" -file " + jbossDir + "/che.crt -storepass " + trustpass + " -noprompt; fi"
	// certificate retrieved from http call to OpenShift API endpoint
	addOpenShiftAPICrt := "if [ ! -z \"${OPENSHIFT_SELF__SIGNED__CERT}\" ]; then echo \"${OPENSHIFT_SELF__SIGNED__CERT}\" > " + jbossDir + "/openshift.crt && " +
		"keytool -importcert -alias OPENSHIFTAPI" +
		" -keystore " + jbossDir + "/openshift.jks" +
		" -file " + jbossDir + "/openshift.crt -storepass " + trustpass + " -noprompt; fi"
	// certificate mounted into container /var/run/secrets
	addMountedCrt := " keytool -importcert -alias MOUNTEDCRT" +
		" -keystore " + jbossDir + "/openshift.jks" +
		" -file /var/run/secrets/kubernetes.io/serviceaccount/ca.crt -storepass " + trustpass + " -noprompt"
	addMountedServiceCrt := "if [ -f /var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt ]; then " +
		"keytool -importcert -alias MOUNTEDSERVICECRT" +
		" -keystore " + jbossDir + "/openshift.jks" +
		" -file /var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt -storepass " + trustpass + " -noprompt; fi"
	importJavaCacerts := "keytool -importkeystore -srckeystore /etc/pki/ca-trust/extracted/java/cacerts" +
		" -destkeystore " + jbossDir + "/openshift.jks" +
		" -srcstorepass changeit -deststorepass " + trustpass

	customPublicCertsDir := "/public-certs"
	customPublicCertsVolumeSource := corev1.VolumeSource{}
	customPublicCertsVolumeSource = corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: tls.CheAllCACertsConfigMapName,
			},
		},
	}
	customPublicCertsVolume := corev1.Volume{
		Name:         "che-public-certs",
		VolumeSource: customPublicCertsVolumeSource,
	}
	customPublicCertsVolumeMount := corev1.VolumeMount{
		Name:      "che-public-certs",
		MountPath: customPublicCertsDir,
	}
	addCustomPublicCertsCommand := "if [[ -d \"" + customPublicCertsDir + "\" && -n \"$(find " + customPublicCertsDir + " -type f)\" ]]; then " +
		"for certfile in " + customPublicCertsDir + "/* ; do " +
		"jks_import_ca_bundle $certfile " + jbossDir + "/openshift.jks " + trustpass + " ; " +
		"done; fi"

	bashFunctions += getImportCABundleScript()
	addCertToTrustStoreCommand := addRouterCrt + " && " + addOpenShiftAPICrt + " && " + addMountedCrt + " && " + addMountedServiceCrt + " && " + importJavaCacerts + " && " + addCustomPublicCertsCommand

	// upstream Keycloak has a bit different mechanism of adding jks
	changeConfigCommand := "echo Installing certificates into Keycloak && " +
		"echo -e \"embed-server --server-config=standalone.xml --std-out=echo \n" +
		"/subsystem=keycloak-server/spi=truststore/:add \n" +
		"/subsystem=keycloak-server/spi=truststore/provider=file/:add(properties={file => " +
		"\"" + jbossDir + "/openshift.jks\", password => \"" + trustpass + "\", disabled => \"false\" },enabled=true) \n" +
		"stop-embedded-server\" > /scripts/add_openshift_certificate.cli && " +
		"/opt/jboss/keycloak/bin/jboss-cli.sh --file=/scripts/add_openshift_certificate.cli"

	addProxyCliCommand := ""
	applyProxyCliCommand := ""
	proxyEnvVars := []corev1.EnvVar{}

	if deployContext.Proxy.HttpProxy != "" {
		proxyEnvVars = []corev1.EnvVar{
			{
				Name:  "HTTP_PROXY",
				Value: deployContext.Proxy.HttpProxy,
			},
			{
				Name:  "HTTPS_PROXY",
				Value: deployContext.Proxy.HttpsProxy,
			},
			{
				Name:  "NO_PROXY",
				Value: deployContext.Proxy.NoProxy,
			},
		}

		quotedNoProxy := ""
		for _, noProxyHost := range strings.Split(deployContext.Proxy.NoProxy, ",") {
			if len(quotedNoProxy) != 0 {
				quotedNoProxy += ","
			}

			var noProxyEntry string
			if strings.HasPrefix(noProxyHost, ".") {
				noProxyEntry = ".*" + strings.ReplaceAll(regexp.QuoteMeta(noProxyHost), "\\", "\\\\\\")
			} else if strings.HasPrefix(noProxyHost, "*.") {
				noProxyEntry = strings.TrimPrefix(noProxyHost, "*")
				noProxyEntry = ".*" + strings.ReplaceAll(regexp.QuoteMeta(noProxyEntry), "\\", "\\\\\\")
			} else {
				noProxyEntry = strings.ReplaceAll(regexp.QuoteMeta(noProxyHost), "\\", "\\\\\\")
			}
			quotedNoProxy += "\"" + noProxyEntry + ";NO_PROXY\""
		}

		serverConfig := "standalone.xml"
		if cheFlavor == "codeready" {
			serverConfig = "standalone-openshift.xml"
		}
		addProxyCliCommand = " && echo Configuring Proxy && " +
			"echo -e 'embed-server --server-config=" + serverConfig + " --std-out=echo \n" +
			"/subsystem=keycloak-server/spi=connectionsHttpClient/provider=default:write-attribute(name=properties.proxy-mappings,value=[" + quotedNoProxy + ",\".*;" + deployContext.Proxy.HttpProxy + "\"]) \n" +
			"stop-embedded-server' > " + jbossDir + "/setup-http-proxy.cli"

		applyProxyCliCommand = " && " + jbossCli + " --file=" + jbossDir + "/setup-http-proxy.cli"
		if cheFlavor == "codeready" {
			applyProxyCliCommand = " && mkdir -p " + jbossDir + "/extensions && echo '#!/bin/bash\n" +
				"" + jbossDir + "/bin/jboss-cli.sh --file=" + jbossDir + "/setup-http-proxy.cli' > " + jbossDir + "/extensions/postconfigure.sh && " +
				"chmod a+x " + jbossDir + "/extensions/postconfigure.sh "
		}
	}

	keycloakEnv := []corev1.EnvVar{
		{
			Name:  "CM_REVISION",
			Value: cmResourceVersions,
		},
		{
			Name:  "PROXY_ADDRESS_FORWARDING",
			Value: "true",
		},
		{
			Name:  "DB_VENDOR",
			Value: "POSTGRES",
		},
		{
			Name:  "DB_USERNAME",
			Value: "keycloak",
		},
		{
			Name:  "DB_ADDR",
			Value: util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName),
		},
		{
			Name:  "DB_DATABASE",
			Value: "keycloak",
		},
		{
			Name:  "POSTGRES_PORT_5432_TCP_ADDR",
			Value: util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName),
		},
		{
			Name:  "POSTGRES_PORT_5432_TCP_PORT",
			Value: util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort),
		},
		{
			Name:  "POSTGRES_PORT",
			Value: util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort),
		},
		{
			Name:  "POSTGRES_USER",
			Value: "keycloak",
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
						Name: deploy.CheTLSSelfSignedCertificateSecretName,
					},
					Optional: &optionalEnv,
				},
			},
		},
		{
			Name: "OPENSHIFT_SELF__SIGNED__CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "ca.crt",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openshift-api-crt",
					},
					Optional: &optionalEnv,
				},
			},
		},
	}

	identityProviderPostgresSecret := deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresSecret
	if len(identityProviderPostgresSecret) > 0 {
		keycloakEnv = append(keycloakEnv, corev1.EnvVar{
			Name: "DB_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: identityProviderPostgresSecret,
					},
				},
			},
		})
	} else {
		keycloakEnv = append(keycloakEnv, corev1.EnvVar{
			Name:  "DB_PASSWORD",
			Value: deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresPassword,
		})
	}

	identityProviderSecret := deployContext.CheCluster.Spec.Auth.IdentityProviderSecret
	if len(identityProviderSecret) > 0 {
		keycloakEnv = append(keycloakEnv, corev1.EnvVar{
			Name: "KEYCLOAK_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: identityProviderSecret,
					},
				},
			},
		},
			corev1.EnvVar{
				Name: "KEYCLOAK_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "user",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: identityProviderSecret,
						},
					},
				},
			})
	} else {
		keycloakEnv = append(keycloakEnv, corev1.EnvVar{
			Name:  "KEYCLOAK_PASSWORD",
			Value: deployContext.CheCluster.Spec.Auth.IdentityProviderPassword,
		},
			corev1.EnvVar{
				Name:  "KEYCLOAK_USER",
				Value: deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName,
			})
	}

	if cheFlavor == "codeready" {
		keycloakEnv = []corev1.EnvVar{
			{
				Name:  "CM_REVISION",
				Value: cmResourceVersions,
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
				Value: util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresHostName, deploy.DefaultChePostgresHostName),
			},
			{
				Name:  "KEYCLOAK_POSTGRESQL_SERVICE_PORT",
				Value: util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresPort, deploy.DefaultChePostgresPort),
			},
			{
				Name:  "DB_DATABASE",
				Value: "keycloak",
			},
			{
				Name:  "DB_USERNAME",
				Value: "keycloak",
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
							Name: deploy.CheTLSSelfSignedCertificateSecretName,
						},
						Optional: &optionalEnv,
					},
				},
			},
			{
				Name: "OPENSHIFT_SELF__SIGNED__CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "ca.crt",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "openshift-api-crt",
						},
						Optional: &optionalEnv,
					},
				},
			},
		}

		identityProviderPostgresSecret := deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresSecret
		if len(identityProviderPostgresSecret) > 0 {
			keycloakEnv = append(keycloakEnv, corev1.EnvVar{
				Name: "DB_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "password",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: identityProviderPostgresSecret,
						},
					},
				},
			})
		} else {
			keycloakEnv = append(keycloakEnv, corev1.EnvVar{
				Name:  "DB_PASSWORD",
				Value: deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresPassword,
			})
		}

		identityProviderSecret := deployContext.CheCluster.Spec.Auth.IdentityProviderSecret
		if len(identityProviderSecret) > 0 {
			keycloakEnv = append(keycloakEnv, corev1.EnvVar{
				Name: "SSO_ADMIN_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "password",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: identityProviderSecret,
						},
					},
				},
			},
				corev1.EnvVar{
					Name: "SSO_ADMIN_USERNAME",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "user",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: identityProviderSecret,
							},
						},
					},
				})
		} else {
			keycloakEnv = append(keycloakEnv, corev1.EnvVar{
				Name:  "SSO_ADMIN_PASSWORD",
				Value: deployContext.CheCluster.Spec.Auth.IdentityProviderPassword,
			},
				corev1.EnvVar{
					Name:  "SSO_ADMIN_USERNAME",
					Value: deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName,
				})
		}
	}

	// Mount GITHUB_CLIENT_ID and GITHUB_SECRET to keycloak container
	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "github",
	})

	if err != nil {
		return nil, err
	} else if len(secrets) == 1 {
		keycloakEnv = append(keycloakEnv, corev1.EnvVar{
			Name: "GITHUB_CLIENT_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "id",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secrets[0].Name,
					},
				},
			},
		}, corev1.EnvVar{
			Name: "GITHUB_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "secret",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secrets[0].Name,
					},
				},
			},
		})
	}

	for _, envvar := range proxyEnvVars {
		keycloakEnv = append(keycloakEnv, envvar)
	}

	// Enable internal network for keycloak
	if deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() && !deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider {
		keycloakEnv = append(keycloakEnv, corev1.EnvVar{
			Name:  "KEYCLOAK_FRONTEND_URL",
			Value: deployContext.CheCluster.Status.KeycloakURL,
		})
	}

	evaluateKeycloakSystemProperties := "KEYCLOAK_SYS_PROPS=\"-Dkeycloak.profile.feature.token_exchange=enabled -Dkeycloak.profile.feature.admin_fine_grained_authz=enabled\""
	if cheFlavor == "codeready" {
		// We need to export `KEYCLOAK_SYS_PROPS` to pass them to standalone.sh via openshift-launch.sh
		evaluateKeycloakSystemProperties = "export KEYCLOAK_SYS_PROPS=\"\""
	}

	// Evaluating keycloak.connectionsHttpClient.default system properties, see details: https://github.com/eclipse/che/issues/19653
	evaluateExpectContinueEnabled := "if [[ $KEYCLOAK_CONNECTIONS_HTTP_CLIENT_DEFAULT_EXPECT_CONTINUE_ENABLED != false ]]; then KEYCLOAK_SYS_PROPS=$KEYCLOAK_SYS_PROPS\" -Dkeycloak.connectionsHttpClient.default.expect-continue-enabled=true\"; fi"
	evaluateReuseConnections := "if [[ $KEYCLOAK_CONNECTIONS_HTTP_CLIENT_DEFAULT_REUSE_CONNECTIONS != true ]]; then KEYCLOAK_SYS_PROPS=$KEYCLOAK_SYS_PROPS\" -Dkeycloak.connectionsHttpClient.default.reuse-connections=false\"; fi"

	command := bashFunctions + "\n" +
		addCertToTrustStoreCommand +
		addProxyCliCommand +
		applyProxyCliCommand +
		" && " + evaluateKeycloakSystemProperties +
		" && " + evaluateExpectContinueEnabled +
		" && " + evaluateReuseConnections +
		" && " + changeConfigCommand +
		" && /opt/jboss/tools/docker-entrypoint.sh -b 0.0.0.0 -c standalone.xml $KEYCLOAK_SYS_PROPS"

	if cheFlavor == "codeready" {
		addUsernameReadonlyTheme := "baseTemplate=/opt/eap/themes/base/login/login-update-profile.ftl" +
			" && readOnlyTemplateDir=/opt/eap/themes/codeready-username-readonly/login" +
			" && readOnlyTemplate=${readOnlyTemplateDir}/login-update-profile.ftl" +
			" && if [ ! -d ${readOnlyTemplateDir} ]; then" +
			" mkdir -p ${readOnlyTemplateDir}" +
			" && cp ${baseTemplate} ${readOnlyTemplate}" +
			" && echo \"parent=rh-sso\" > ${readOnlyTemplateDir}/theme.properties" +
			" && sed -i 's|id=\"username\" name=\"username\"|id=\"username\" readonly name=\"username\"|g' ${readOnlyTemplate}; fi"
		addUsernameValidationForKeycloakTheme := "sed -i  's|id=\"username\" name=\"username\"|" +
			"id=\"username\" " +
			"pattern=\"[a-z]([-a-z0-9]{0,61}[a-z0-9])?\" " +
			"title=\"Username has to comply with the DNS naming convention. An alphanumeric (a-z, and 0-9) string, with a maximum length of 63 characters, with the '-' character allowed anywhere except the first or last character.\" " +
			"name=\"username\"|g' ${baseTemplate}"
		patchOpenshiftLaunch := "sed -i 's|standalone.sh -c standalone-openshift.xml|standalone.sh -c standalone-openshift.xml ${KEYCLOAK_SYS_PROPS}|' /opt/eap/bin/openshift-launch.sh"
		command = bashFunctions + "\n" +
			addUsernameReadonlyTheme +
			" && " + addUsernameValidationForKeycloakTheme +
			" && " + addCertToTrustStoreCommand +
			addProxyCliCommand +
			applyProxyCliCommand +
			" && " + evaluateKeycloakSystemProperties +
			" && " + evaluateExpectContinueEnabled +
			" && " + evaluateReuseConnections +
			" && " + patchOpenshiftLaunch +
			" && echo \"feature.token_exchange=enabled\nfeature.admin_fine_grained_authz=enabled\" > /opt/eap/standalone/configuration/profile.properties  " +
			" && sed -i 's/WILDCARD/ANY/g' /opt/eap/bin/launch/keycloak-spi.sh && /opt/eap/bin/openshift-launch.sh -b 0.0.0.0"
	}

	sslRequiredUpdatedForMasterRealm := isSslRequiredUpdatedForMasterRealm(deployContext)
	if sslRequiredUpdatedForMasterRealm {
		// update command to restart pod
		command = "echo \"ssl_required WAS UPDATED for master realm.\" && " + command
	}

	ports := []corev1.ContainerPort{
		{
			Name:          deploy.IdentityProviderName,
			ContainerPort: 8080,
			Protocol:      "TCP",
		},
	}

	if deployContext.CheCluster.Spec.Auth.Debug {
		ports = append(ports, corev1.ContainerPort{
			Name:          "debug",
			ContainerPort: 8787,
			Protocol:      "TCP",
		})

		keycloakEnv = append(keycloakEnv, []corev1.EnvVar{
			{
				Name:  "DEBUG",
				Value: "true",
			},
			{
				Name:  "DEBUG_PORT",
				Value: "*:8787",
			},
		}...)

		command += " --debug"
	}

	args := []string{"-c", command}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.IdentityProviderName,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"che.self-signed-certificate.version": cheCertSecretVersion,
				"che.openshift-api-crt.version":       openshiftApiCertSecretVersion,
				"che.keycloak-ssl-required-updated":   strconv.FormatBool(sslRequiredUpdatedForMasterRealm),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labelSelector},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						customPublicCertsVolume,
					},
					Containers: []corev1.Container{
						{
							Name:            deploy.IdentityProviderName,
							Image:           keycloakImage,
							ImagePullPolicy: pullPolicy,
							Command: []string{
								"/bin/sh",
							},
							Args:  args,
							Ports: ports,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Auth.IdentityProviderContainerResources.Requests.Memory,
										deploy.DefaultIdentityProviderMemoryRequest),
									corev1.ResourceCPU: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Auth.IdentityProviderContainerResources.Requests.Cpu,
										deploy.DefaultIdentityProviderCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Auth.IdentityProviderContainerResources.Limits.Memory,
										deploy.DefaultIdentityProviderMemoryLimit),
									corev1.ResourceCPU: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Auth.IdentityProviderContainerResources.Limits.Cpu,
										deploy.DefaultIdentityProviderCpuLimit),
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
								InitialDelaySeconds: 30,
								FailureThreshold:    10,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 300,
								FailureThreshold:    10,
								TimeoutSeconds:      5,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Env: keycloakEnv,
							VolumeMounts: []corev1.VolumeMount{
								customPublicCertsVolumeMount,
							},
						},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					RestartPolicy:                 "Always",
				},
			},
		},
	}

	if deploy.IsComponentReadinessInitContainersConfigured(deployContext.CheCluster) {
		if !deployContext.CheCluster.Spec.Database.ExternalDb {
			waitForPostgresInitContainer, err := postgres.GetWaitForPostgresInitContainer(deployContext)
			if err != nil {
				return nil, err
			}
			deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *waitForPostgresInitContainer)
		}
	}

	return deployment, nil
}

func getSecretResourceVersion(name string, namespace string, clusterAPI deploy.ClusterAPI) string {
	secret := &corev1.Secret{}
	err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, secret)
	if err != nil {
		if !errors.IsNotFound(err) {
			logrus.Errorf("Failed to get %s secret: %s", name, err)
		}
		return ""
	}
	return secret.ResourceVersion
}

func isSslRequiredUpdatedForMasterRealm(deployContext *deploy.DeployContext) bool {
	if deployContext.CheCluster.Spec.Database.ExternalDb {
		return false
	}

	if util.IsTestMode() {
		return false
	}

	actual := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(deployContext, deploy.IdentityProviderName, actual)
	if !exists || err != nil {
		if err != nil {
			logrus.Error(err)
		}
		return false
	}

	value, err := strconv.ParseBool(actual.ObjectMeta.Annotations["che.keycloak-ssl-required-updated"])
	if err == nil && value {
		return true
	}

	dbValue, _ := getSslRequiredForMasterRealm(deployContext.CheCluster)
	return dbValue == "NONE"
}

func getSslRequiredForMasterRealm(cr *orgv1.CheCluster) (string, error) {
	stdout, err := util.K8sclient.ExecIntoPod(
		cr,
		deploy.PostgresName,
		func(cr *orgv1.CheCluster) (string, error) {
			return selectSslRequiredCommand, nil
		},
		"")
	return strings.TrimSpace(stdout), err
}

func updateSslRequiredForMasterRealm(cr *orgv1.CheCluster) error {
	_, err := util.K8sclient.ExecIntoPod(
		cr,
		deploy.PostgresName,
		func(cr *orgv1.CheCluster) (string, error) {
			return updateSslRequiredCommand, nil
		},
		"Update ssl_required to NONE")
	return err
}

func ProvisionKeycloakResources(deployContext *deploy.DeployContext) error {
	if !deployContext.CheCluster.Spec.Database.ExternalDb {
		value, err := getSslRequiredForMasterRealm(deployContext.CheCluster)
		if err != nil {
			return err
		}

		if value != "NONE" {
			err := updateSslRequiredForMasterRealm(deployContext.CheCluster)
			if err != nil {
				return err
			}
		}
	}

	_, err := util.K8sclient.ExecIntoPod(
		deployContext.CheCluster,
		deploy.IdentityProviderName,
		GetKeycloakProvisionCommand,
		"create realm, client and user")
	return err
}

// getImportCABundleScript returns string which contains bash function that imports ca-bundle into jks
// The function has three arguments:
// - absolute path to ca-bundle file
// - absolute path to java keystore
// - java keystore password
func getImportCABundleScript() string {
	return `
	function jks_import_ca_bundle {
		CA_FILE=$1
		KEYSTORE_PATH=$2
		KEYSTORE_PASSWORD=$3

		if [ ! -f $CA_FILE ]; then
			# CA bundle file doesn't exist, skip it
			echo "Failed to import CA certificates from ${CA_FILE}. File doesn't exist"
			return
		fi

		bundle_name=$(basename $CA_FILE)
		certs_imported=0
		cert_index=0
		tmp_file=/tmp/cert.pem
		is_cert=false
		while IFS= read -r line; do
			if [ "$line" == "-----BEGIN CERTIFICATE-----" ]; then
				# Start copying a new certificate
				is_cert=true
				cert_index=$((cert_index+1))
				# Reset destination file and add header line
				echo $line > ${tmp_file}
			elif [ "$line" == "-----END CERTIFICATE-----" ]; then
				# End of the certificate is reached, add it to trust store
				is_cert=false
				echo $line >> ${tmp_file}
				keytool -importcert -alias "${bundle_name}_${cert_index}" -keystore $KEYSTORE_PATH -file $tmp_file -storepass $KEYSTORE_PASSWORD -noprompt && \
				certs_imported=$((certs_imported+1))
			elif [ "$is_cert" == true ]; then
				# In the middle of a certificate, copy line to target file
				echo $line >> ${tmp_file}
			fi
		done < "$CA_FILE"
		echo "Imported ${certs_imported} certificates from ${CA_FILE}"
		# Clean up
		rm -f $tmp_file
	}
	`
}
