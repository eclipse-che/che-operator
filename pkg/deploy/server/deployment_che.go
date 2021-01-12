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
package server

import (
	"strconv"
	"strings"

	"github.com/eclipse/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncCheDeploymentToCluster(deployContext *deploy.DeployContext) deploy.DeploymentProvisioningStatus {
	clusterDeployment, err := deploy.GetClusterDeployment(deploy.DefaultCheFlavor(deployContext.CheCluster), deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := getSpecCheDeployment(deployContext)
	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	return deploy.SyncDeploymentToCluster(deployContext, specDeployment, clusterDeployment, nil, nil)
}

func getSpecCheDeployment(deployContext *deploy.DeployContext) (*appsv1.Deployment, error) {
	isOpenShift, _, err := util.DetectOpenShift()
	if err != nil {
		return nil, err
	}

	selfSignedCertUsed, err := deploy.IsSelfSignedCertificateUsed(deployContext)
	if err != nil {
		return nil, err
	}

	cmResourceVersions := GetCheConfigMapVersion(deployContext)
	cmResourceVersions += "," + deploy.GetAdditionalCACertsConfigMapVersion(deployContext)

	terminationGracePeriodSeconds := int64(30)
	cheFlavor := deploy.DefaultCheFlavor(deployContext.CheCluster)
	labels, labelSelector := deploy.GetLabelsAndSelector(deployContext.CheCluster, cheFlavor)
	optionalEnv := true
	memRequest := util.GetValue(deployContext.CheCluster.Spec.Server.ServerMemoryRequest, deploy.DefaultServerMemoryRequest)
	selfSignedCertEnv := corev1.EnvVar{
		Name:  "CHE_SELF__SIGNED__CERT",
		Value: "",
	}
	customPublicCertsVolumeSource := corev1.VolumeSource{}
	customPublicCertsVolumeSource = corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: deploy.CheAllCACertsConfigMapName,
			},
		},
	}
	customPublicCertsVolume := corev1.Volume{
		Name:         "che-public-certs",
		VolumeSource: customPublicCertsVolumeSource,
	}
	customPublicCertsVolumeMount := corev1.VolumeMount{
		Name:      "che-public-certs",
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
	if selfSignedCertUsed {
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
	if deployContext.CheCluster.Spec.Server.GitSelfSignedCert {
		gitSelfSignedCertEnv = corev1.EnvVar{
			Name: "CHE_GIT_SELF__SIGNED__CERT",
			ValueFrom: &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					Key: "ca.crt",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "che-git-self-signed-cert",
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
						Name: "che-git-self-signed-cert",
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

	identityProviderSecret := deployContext.CheCluster.Spec.Auth.IdentityProviderSecret
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
			Value: deployContext.CheCluster.Spec.Auth.IdentityProviderPassword,
		},
			corev1.EnvVar{
				Name:  "CHE_KEYCLOAK_ADMIN__USERNAME",
				Value: deployContext.CheCluster.Spec.Auth.IdentityProviderAdminUserName,
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

	memLimit := util.GetValue(deployContext.CheCluster.Spec.Server.ServerMemoryLimit, deploy.DefaultServerMemoryLimit)
	cheImageAndTag := GetFullCheServerImageLink(deployContext.CheCluster)
	pullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Server.CheImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(cheImageAndTag)))

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheFlavor,
			Namespace: deployContext.CheCluster.Namespace,
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
									corev1.ResourceMemory: resource.MustParse(memRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memLimit),
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

	cheMultiUser := deploy.GetCheMultiUser(deployContext.CheCluster)
	if cheMultiUser == "true" {
		chePostgresSecret := deployContext.CheCluster.Spec.Database.ChePostgresSecret
		if len(chePostgresSecret) > 0 {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env,
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
	} else {
		deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: deploy.DefaultCheVolumeClaimName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: deploy.DefaultCheVolumeClaimName,
					},
				},
			}}
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: deploy.DefaultCheVolumeMountPath,
				Name:      deploy.DefaultCheVolumeClaimName,
			}}
	}

	// configure probes if debug isn't set
	cheDebug := util.GetValue(deployContext.CheCluster.Spec.Server.CheDebug, deploy.DefaultCheDebug)
	if cheDebug != "true" {
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
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
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
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
			InitialDelaySeconds: 200,
			FailureThreshold:    3,
			TimeoutSeconds:      3,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
		}
	}

	if !isOpenShift {
		runAsUser, err := strconv.ParseInt(util.GetValue(deployContext.CheCluster.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser), 10, 64)
		if err != nil {
			return nil, err
		}
		fsGroup, err := strconv.ParseInt(util.GetValue(deployContext.CheCluster.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup), 10, 64)
		if err != nil {
			return nil, err
		}
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &fsGroup,
		}
	}

	if !util.IsTestMode() {
		err = controllerutil.SetControllerReference(deployContext.CheCluster, deployment, deployContext.ClusterAPI.Scheme)
		if err != nil {
			return nil, err
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
