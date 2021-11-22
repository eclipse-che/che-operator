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
package server

import (
	"errors"
	"strconv"
	"strings"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/deploy/postgres"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s Server) getDeploymentSpec() (*appsv1.Deployment, error) {
	selfSignedCASecretExists, err := tls.IsSelfSignedCASecretExists(s.deployContext)
	if err != nil {
		return nil, err
	}

	cmResourceVersions := GetCheConfigMapVersion(s.deployContext)
	cmResourceVersions += "," + tls.GetAdditionalCACertsConfigMapVersion(s.deployContext)

	terminationGracePeriodSeconds := int64(30)
	cheFlavor := deploy.DefaultCheFlavor(s.deployContext.CheCluster)
	labels, labelSelector := deploy.GetLabelsAndSelector(s.deployContext.CheCluster, cheFlavor)
	optionalEnv := true
	selfSignedCertEnv := corev1.EnvVar{
		Name:  "CHE_SELF__SIGNED__CERT",
		Value: "",
	}
	customPublicCertsVolume := corev1.Volume{
		Name: "che-public-certs",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tls.CheAllCACertsConfigMapName,
				},
			},
		},
	}
	customPublicCertsVolumeMount := corev1.VolumeMount{
		Name:      customPublicCertsVolume.Name,
		MountPath: "/public-certs",
	}
	gitSelfSignedCertEnv := corev1.EnvVar{
		Name:  "CHE_GIT_SELF__SIGNED__CERT",
		Value: "",
	}
	gitSelfSignedCertHostEnv := corev1.EnvVar{
		Name:  "CHE_GIT_SELF__SIGNED__CERT__HOST",
		Value: "",
	}
	if selfSignedCASecretExists {
		selfSignedCertEnv = corev1.EnvVar{
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
		}
	}
	if s.deployContext.CheCluster.Spec.Server.GitSelfSignedCert {
		gitSelfSignedCertEnv = corev1.EnvVar{
			Name: "CHE_GIT_SELF__SIGNED__CERT",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key: "ca.crt",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: deploy.GitSelfSignedCertsConfigMapName,
					},
					Optional: &optionalEnv,
				},
			},
		}
		gitSelfSignedCertHostEnv = corev1.EnvVar{
			Name: "CHE_GIT_SELF__SIGNED__CERT__HOST",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key: "githost",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: deploy.GitSelfSignedCertsConfigMapName,
					},
					Optional: &optionalEnv,
				},
			},
		}
	}

	var cheEnv []corev1.EnvVar
	cheEnv = append(cheEnv, selfSignedCertEnv)
	cheEnv = append(cheEnv, gitSelfSignedCertEnv)
	cheEnv = append(cheEnv, gitSelfSignedCertHostEnv)

	identityProviderSecret := s.deployContext.CheCluster.Spec.Auth.IdentityProviderSecret
	if len(identityProviderSecret) > 0 {
		cheEnv = append(cheEnv, corev1.EnvVar{
			Name: "CHE_KEYCLOAK_ADMIN__PASSWORD",
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
				Name: "CHE_KEYCLOAK_ADMIN__USERNAME",
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
		cheEnv = append(cheEnv, corev1.EnvVar{
			Name:  "CHE_KEYCLOAK_ADMIN__PASSWORD",
			Value: s.deployContext.CheCluster.Spec.Auth.IdentityProviderPassword,
		},
			corev1.EnvVar{
				Name:  "CHE_KEYCLOAK_ADMIN__USERNAME",
				Value: s.deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName,
			})
	}

	cheEnv = append(cheEnv,
		corev1.EnvVar{
			Name:  "CM_REVISION",
			Value: cmResourceVersions,
		},
		corev1.EnvVar{
			Name: "KUBERNETES_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace"}},
		})

	if util.IsOpenShift && s.deployContext.CheCluster.IsNativeUserModeEnabled() {
		cheEnv = append(cheEnv, corev1.EnvVar{
			Name:  "CHE_AUTH_NATIVEUSER",
			Value: "true",
		})
	}

	cheImageAndTag := GetFullCheServerImageLink(s.deployContext.CheCluster)
	pullPolicy := corev1.PullPolicy(util.GetValue(string(s.deployContext.CheCluster.Spec.Server.CheImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(cheImageAndTag)))

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheFlavor,
			Namespace: s.deployContext.CheCluster.Namespace,
			Labels:    labels,
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
					ServiceAccountName:       "che",
					DeprecatedServiceAccount: "che",
					Volumes: []corev1.Volume{
						customPublicCertsVolume,
					},
					Containers: []corev1.Container{
						{
							Name:            cheFlavor,
							ImagePullPolicy: pullPolicy,
							Image:           cheImageAndTag,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
								{
									Name:          "http-debug",
									ContainerPort: 8000,
									Protocol:      "TCP",
								},
								{
									Name:          "jgroups-ping",
									ContainerPort: 8888,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										s.deployContext.CheCluster.Spec.Server.ServerMemoryRequest,
										deploy.DefaultServerMemoryRequest),
									corev1.ResourceCPU: util.GetResourceQuantity(
										s.deployContext.CheCluster.Spec.Server.ServerCpuRequest,
										deploy.DefaultServerCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										s.deployContext.CheCluster.Spec.Server.ServerMemoryLimit,
										deploy.DefaultServerMemoryLimit),
									corev1.ResourceCPU: util.GetResourceQuantity(
										s.deployContext.CheCluster.Spec.Server.ServerCpuLimit,
										deploy.DefaultServerCpuLimit),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: "che"},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								customPublicCertsVolumeMount,
							},
							Env: cheEnv,
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	err = MountBitBucketOAuthConfig(s.deployContext, deployment)
	if err != nil {
		return nil, err
	}

	err = MountGitHubOAuthConfig(s.deployContext, deployment)
	if err != nil {
		return nil, err
	}

	err = MountGitLabOAuthConfig(s.deployContext, deployment)
	if err != nil {
		return nil, err
	}

	container := &deployment.Spec.Template.Spec.Containers[0]
	chePostgresSecret := s.deployContext.CheCluster.Spec.Database.ChePostgresSecret
	if len(chePostgresSecret) > 0 {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name: "CHE_JDBC_USERNAME",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "user",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: chePostgresSecret,
						},
					},
				},
			}, corev1.EnvVar{
				Name: "CHE_JDBC_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "password",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: chePostgresSecret,
						},
					},
				},
			})
	}

	// configure probes if debug isn't set
	cheDebug := util.GetValue(s.deployContext.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	if cheDebug != "true" {
		container.ReadinessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/api/system/state",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(8080),
					},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			// After POD start, the POD will be seen as ready after a minimum of 15 seconds and we expect it to be seen as ready until a maximum of 200 seconds
			// 200 s = InitialDelaySeconds + PeriodSeconds * (FailureThreshold - 1) + TimeoutSeconds
			InitialDelaySeconds: 25,
			FailureThreshold:    18,
			TimeoutSeconds:      5,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
		}
		container.LivenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/api/system/state",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(8080),
					},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			// After POD start, don't initiate liveness probe while the POD is still expected to be declared as ready by the readiness probe
			InitialDelaySeconds: 400,
			FailureThreshold:    3,
			TimeoutSeconds:      3,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
		}
	}

	if !util.IsOpenShift {
		runAsUser, err := strconv.ParseInt(util.GetValue(s.deployContext.CheCluster.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser), 10, 64)
		if err != nil {
			return nil, err
		}
		fsGroup, err := strconv.ParseInt(util.GetValue(s.deployContext.CheCluster.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup), 10, 64)
		if err != nil {
			return nil, err
		}
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &fsGroup,
		}
	}

	if deploy.IsComponentReadinessInitContainersConfigured(s.deployContext.CheCluster) {
		if !s.deployContext.CheCluster.Spec.Database.ExternalDb {
			waitForPostgresInitContainer, err := postgres.GetWaitForPostgresInitContainer(s.deployContext)
			if err != nil {
				return nil, err
			}
			deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *waitForPostgresInitContainer)
		}

		if !s.deployContext.CheCluster.Spec.Auth.ExternalIdentityProvider {
			waitForKeycloakInitContainer, err := identity_provider.GetWaitForKeycloakInitContainer(s.deployContext)
			if err != nil {
				return nil, err
			}
			deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *waitForKeycloakInitContainer)
		}
	}

	return deployment, nil
}

// GetFullCheServerImageLink evaluate full cheImage link(with repo and tag)
// based on Checluster information and image defaults from env variables
func GetFullCheServerImageLink(checluster *orgv1.CheCluster) string {
	if len(checluster.Spec.Server.CheImage) > 0 {
		cheServerImageTag := util.GetValue(checluster.Spec.Server.CheImageTag, deploy.DefaultCheVersion())
		return checluster.Spec.Server.CheImage + ":" + cheServerImageTag
	}

	defaultCheServerImage := deploy.DefaultCheServerImage(checluster)
	if len(checluster.Spec.Server.CheImageTag) == 0 {
		return defaultCheServerImage
	}

	// For back compatibility with version < 7.9.0:
	// if cr.Spec.Server.CheImage is empty, but cr.Spec.Server.CheImageTag is not empty,
	// parse from default Che image(value comes from env variable) "Che image repository"
	// and return "Che image", like concatenation: "cheImageRepo:cheImageTag"
	separator := map[bool]string{true: "@", false: ":"}[strings.Contains(defaultCheServerImage, "@")]
	imageParts := strings.Split(defaultCheServerImage, separator)
	return imageParts[0] + ":" + checluster.Spec.Server.CheImageTag
}

func MountBitBucketOAuthConfig(deployContext *deploy.DeployContext, deployment *appsv1.Deployment) error {
	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "bitbucket",
	})

	if err != nil {
		return err
	} else if len(secrets) > 1 {
		return errors.New("More than 1 BitBucket OAuth configuration secrets found")
	} else if len(secrets) == 1 {
		mountSecret(deployment, &secrets[0], deploy.BitBucketOAuthConfigMountPath)
		mountEnv(deployment, []corev1.EnvVar{
			{
				Name:  "CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH",
				Value: deploy.BitBucketOAuthConfigMountPath + "/" + deploy.BitBucketOAuthConfigConsumerKeyFileName,
			}, {
				Name:  "CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH",
				Value: deploy.BitBucketOAuthConfigMountPath + "/" + deploy.BitBucketOAuthConfigPrivateKeyFileName,
			},
		})

		endpoint := secrets[0].Annotations[deploy.CheEclipseOrgScmServerEndpoint]
		if endpoint != "" {
			mountEnv(deployment, []corev1.EnvVar{{
				Name:  "CHE_OAUTH1_BITBUCKET_ENDPOINT",
				Value: endpoint,
			}})
		}
	}

	return nil
}

func MountGitHubOAuthConfig(deployContext *deploy.DeployContext, deployment *appsv1.Deployment) error {
	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "github",
	})

	if err != nil {
		return err
	} else if len(secrets) > 1 {
		return errors.New("More than 1 GitHub OAuth configuration secrets found")
	} else if len(secrets) == 1 {
		mountSecret(deployment, &secrets[0], deploy.GitHubOAuthConfigMountPath)
		mountEnv(deployment, []corev1.EnvVar{
			{
				Name:  "CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH",
				Value: deploy.GitHubOAuthConfigMountPath + "/" + deploy.GitHubOAuthConfigClientIdFileName,
			}, {
				Name:  "CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH",
				Value: deploy.GitHubOAuthConfigMountPath + "/" + deploy.GitHubOAuthConfigClientSecretFileName,
			},
		})

		endpoint := secrets[0].Annotations[deploy.CheEclipseOrgScmServerEndpoint]
		if endpoint != "" {
			mountEnv(deployment, []corev1.EnvVar{{
				Name:  "CHE_INTEGRATION_GITHUB_SERVER__ENDPOINTS",
				Value: endpoint,
			}})
		}
	}

	return nil
}

func MountGitLabOAuthConfig(deployContext *deploy.DeployContext, deployment *appsv1.Deployment) error {
	secrets, err := deploy.GetSecrets(deployContext, map[string]string{
		deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
		deploy.KubernetesComponentLabelKey: deploy.OAuthScmConfiguration,
	}, map[string]string{
		deploy.CheEclipseOrgOAuthScmServer: "gitlab",
	})

	if err != nil {
		return err
	} else if len(secrets) > 1 {
		return errors.New("More than 1 GitLab OAuth configuration secrets found")
	} else if len(secrets) == 1 {
		mountSecret(deployment, &secrets[0], deploy.GitLabOAuthConfigMountPath)
		mountEnv(deployment, []corev1.EnvVar{
			{
				Name:  "CHE_OAUTH_GITLAB_CLIENTID__FILEPATH",
				Value: deploy.GitLabOAuthConfigMountPath + "/" + deploy.GitLabOAuthConfigClientIdFileName,
			}, {
				Name:  "CHE_OAUTH_GITLAB_CLIENTSECRET__FILEPATH",
				Value: deploy.GitLabOAuthConfigMountPath + "/" + deploy.GitLabOAuthConfigClientSecretFileName,
			},
		})

		endpoint := secrets[0].Annotations[deploy.CheEclipseOrgScmServerEndpoint]
		if endpoint != "" {
			mountEnv(deployment, []corev1.EnvVar{{
				Name:  "CHE_INTEGRATION_GITLAB_SERVER__ENDPOINTS",
				Value: endpoint,
			}})
		}
	}

	return nil
}

func mountSecret(deployment *appsv1.Deployment, secret *corev1.Secret, mountPath string) {
	container := &deployment.Spec.Template.Spec.Containers[0]
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: secret.Name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		})
	container.VolumeMounts = append(container.VolumeMounts,
		corev1.VolumeMount{
			Name:      secret.Name,
			MountPath: mountPath,
		})
}

func mountEnv(deployment *appsv1.Deployment, envVar []corev1.EnvVar) {
	container := &deployment.Spec.Template.Spec.Containers[0]
	container.Env = append(container.Env, envVar...)
}
