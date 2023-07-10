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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s CheServerReconciler) getDeploymentSpec(ctx *chetypes.DeployContext) (*appsv1.Deployment, error) {
	selfSignedCASecretExists, err := tls.IsSelfSignedCASecretExists(ctx)
	if err != nil {
		return nil, err
	}

	cmResourceVersions := GetCheConfigMapVersion(ctx)
	cmResourceVersions += "," + tls.GetAdditionalCACertsConfigMapVersion(ctx)

	terminationGracePeriodSeconds := int64(30)
	labels, labelSelector := deploy.GetLabelsAndSelector(defaults.GetCheFlavor())
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

	if selfSignedCASecretExists {
		selfSignedCertEnv = corev1.EnvVar{
			Name: "CHE_SELF__SIGNED__CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "ca.crt",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: constants.DefaultSelfSignedCertificateSecretName,
					},
					Optional: &optionalEnv,
				},
			},
		}
	}

	var cheEnv []corev1.EnvVar
	cheEnv = append(cheEnv, selfSignedCertEnv)
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
	cheEnv = append(cheEnv, corev1.EnvVar{
		Name:  "CHE_AUTH_NATIVEUSER",
		Value: "true",
	})

	image := defaults.GetCheServerImage(ctx.CheCluster)
	pullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaults.GetCheFlavor(),
			Namespace: ctx.CheCluster.Namespace,
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
							Name:            defaults.GetCheFlavor(),
							Image:           image,
							ImagePullPolicy: pullPolicy,
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
									corev1.ResourceMemory: resource.MustParse(constants.DefaultServerMemoryRequest),
									corev1.ResourceCPU:    resource.MustParse(constants.DefaultServerCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.DefaultServerMemoryLimit),
									corev1.ResourceCPU:    resource.MustParse(constants.DefaultServerCpuLimit),
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

	err = MountBitBucketOAuthConfig(ctx, deployment)
	if err != nil {
		return nil, err
	}

	err = MountGitHubOAuthConfig(ctx, deployment)
	if err != nil {
		return nil, err
	}

	err = MountGitLabOAuthConfig(ctx, deployment)
	if err != nil {
		return nil, err
	}

	if err := MountAzureDevOpsOAuthConfig(ctx, deployment); err != nil {
		return nil, err
	}

	container := &deployment.Spec.Template.Spec.Containers[0]

	// configure probes if debug isn't set
	if ctx.CheCluster.Spec.Components.CheServer.Debug == nil || !*ctx.CheCluster.Spec.Components.CheServer.Debug {
		container.ReadinessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
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
			ProbeHandler: corev1.ProbeHandler{
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

	deploy.EnsurePodSecurityStandards(deployment, constants.DefaultSecurityContextRunAsUser, constants.DefaultSecurityContextFsGroup)
	if err := deploy.OverrideDeployment(ctx, deployment, ctx.CheCluster.Spec.Components.CheServer.Deployment); err != nil {
		return nil, err
	}

	return deployment, nil
}

func MountBitBucketOAuthConfig(ctx *chetypes.DeployContext, deployment *appsv1.Deployment) error {
	secret, err := getOAuthConfig(ctx, "bitbucket")
	if secret == nil {
		return err
	}

	mountVolumes(deployment, secret, constants.BitBucketOAuthConfigMountPath)

	if secret.Data["id"] != nil && secret.Data["secret"] != nil {
		mountEnv(deployment, "CHE_OAUTH2_BITBUCKET_CLIENTID__FILEPATH", constants.BitBucketOAuthConfigMountPath+"/"+constants.BitBucketOAuthConfigClientIdFileName)
		mountEnv(deployment, "CHE_OAUTH2_BITBUCKET_CLIENTSECRET__FILEPATH", constants.BitBucketOAuthConfigMountPath+"/"+constants.BitBucketOAuthConfigClientSecretFileName)
	} else {
		mountEnv(deployment, "CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH", constants.BitBucketOAuthConfigMountPath+"/"+constants.BitBucketOAuthConfigConsumerKeyFileName)
		mountEnv(deployment, "CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH", constants.BitBucketOAuthConfigMountPath+"/"+constants.BitBucketOAuthConfigPrivateKeyFileName)
	}

	oauthEndpoint := secret.Annotations[constants.CheEclipseOrgScmServerEndpoint]
	if oauthEndpoint != "" {
		mountEnv(deployment, "CHE_OAUTH_BITBUCKET_ENDPOINT", oauthEndpoint)
	}
	return nil
}

func MountGitHubOAuthConfig(ctx *chetypes.DeployContext, deployment *appsv1.Deployment) error {
	secret, err := getOAuthConfig(ctx, "github")
	if secret == nil {
		return err
	}

	mountVolumes(deployment, secret, constants.GitHubOAuthConfigMountPath)
	mountEnv(deployment, "CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH", constants.GitHubOAuthConfigMountPath+"/"+constants.GitHubOAuthConfigClientIdFileName)
	mountEnv(deployment, "CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH", constants.GitHubOAuthConfigMountPath+"/"+constants.GitHubOAuthConfigClientSecretFileName)

	oauthEndpoint := secret.Annotations[constants.CheEclipseOrgScmServerEndpoint]
	if oauthEndpoint != "" {
		mountEnv(deployment, "CHE_INTEGRATION_GITHUB_OAUTH__ENDPOINT", oauthEndpoint)
	}

	if secret.Annotations[constants.CheEclipseOrgScmGitHubDisableSubdomainIsolation] != "" {
		mountEnv(deployment, "CHE_INTEGRATION_GITHUB_DISABLE__SUBDOMAIN__ISOLATION", secret.Annotations[constants.CheEclipseOrgScmGitHubDisableSubdomainIsolation])
	}

	return nil
}

func MountAzureDevOpsOAuthConfig(ctx *chetypes.DeployContext, deployment *appsv1.Deployment) error {
	secret, err := getOAuthConfig(ctx, constants.AzureDevOpsOAuth)
	if secret == nil {
		return err
	}

	mountVolumes(deployment, secret, constants.AzureDevOpsOAuthConfigMountPath)
	mountEnv(deployment, "CHE_OAUTH2_AZURE_DEVOPS_CLIENTID__FILEPATH", constants.AzureDevOpsOAuthConfigMountPath+"/"+constants.AzureDevOpsOAuthConfigClientIdFileName)
	mountEnv(deployment, "CHE_OAUTH2_AZURE_DEVOPS_CLIENTSECRET__FILEPATH", constants.AzureDevOpsOAuthConfigMountPath+"/"+constants.AzureDevOpsOAuthConfigClientSecretFileName)

	return nil
}

func MountGitLabOAuthConfig(ctx *chetypes.DeployContext, deployment *appsv1.Deployment) error {
	secret, err := getOAuthConfig(ctx, "gitlab")
	if secret == nil {
		return err
	}

	mountVolumes(deployment, secret, constants.GitLabOAuthConfigMountPath)
	mountEnv(deployment, "CHE_OAUTH2_GITLAB_CLIENTID__FILEPATH", constants.GitLabOAuthConfigMountPath+"/"+constants.GitLabOAuthConfigClientIdFileName)
	mountEnv(deployment, "CHE_OAUTH2_GITLAB_CLIENTSECRET__FILEPATH", constants.GitLabOAuthConfigMountPath+"/"+constants.GitLabOAuthConfigClientSecretFileName)

	oauthEndpoint := secret.Annotations[constants.CheEclipseOrgScmServerEndpoint]
	if oauthEndpoint != "" {
		mountEnv(deployment, "CHE_INTEGRATION_GITLAB_OAUTH__ENDPOINT", oauthEndpoint)
	}
	return nil
}

func mountVolumes(deployment *appsv1.Deployment, secret *corev1.Secret, mountPath string) {
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

func mountEnv(deployment *appsv1.Deployment, envName string, envValue string) {
	container := &deployment.Spec.Template.Spec.Containers[0]
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  envName,
		Value: envValue,
	})
}
