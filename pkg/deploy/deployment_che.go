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
	"strconv"
	"strings"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncCheDeploymentToCluster(checluster *orgv1.CheCluster, cmResourceVersion string, clusterAPI ClusterAPI) DeploymentProvisioningStatus {
	clusterDeployment, err := getClusterDeployment(DefaultCheFlavor(checluster), checluster.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := getSpecCheDeployment(checluster, cmResourceVersion, clusterAPI)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	return SyncDeploymentToCluster(checluster, specDeployment, clusterDeployment, nil, nil, clusterAPI)
}

func getSpecCheDeployment(checluster *orgv1.CheCluster, cmResourceVersion string, clusterAPI ClusterAPI) (*appsv1.Deployment, error) {
	isOpenShift, _, err := util.DetectOpenShift()
	if err != nil {
		return nil, err
	}

	selfSignedCertUsed, err := IsSelfSignedCertificateUsed(checluster, clusterAPI)
	if err != nil {
		return nil, err
	}

	terminationGracePeriodSeconds := int64(30)
	cheFlavor := DefaultCheFlavor(checluster)
	labels := GetLabels(checluster, cheFlavor)
	optionalEnv := true
	memRequest := util.GetValue(checluster.Spec.Server.ServerMemoryRequest, DefaultServerMemoryRequest)
	selfSignedCertEnv := corev1.EnvVar{
		Name:  "CHE_SELF__SIGNED__CERT",
		Value: "",
	}
	customPublicCertsVolumeSource := corev1.VolumeSource{}
	if checluster.Spec.Server.ServerTrustStoreConfigMapName != "" {
		customPublicCertsVolumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: checluster.Spec.Server.ServerTrustStoreConfigMapName,
				},
			},
		}
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
						Name: CheTLSSelfSignedCertificateSecretName,
					},
					Optional: &optionalEnv,
				},
			},
		}
	}
	if checluster.Spec.Server.GitSelfSignedCert {
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

	memLimit := util.GetValue(checluster.Spec.Server.ServerMemoryLimit, DefaultServerMemoryLimit)
	cheImageAndTag := GetFullCheServerImageLink(checluster)
	pullPolicy := corev1.PullPolicy(util.GetValue(string(checluster.Spec.Server.CheImagePullPolicy), DefaultPullPolicyFromDockerImage(cheImageAndTag)))

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheFlavor,
			Namespace: checluster.Namespace,
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
							LivenessProbe: &corev1.Probe{
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
							Env: []corev1.EnvVar{
								{
									Name:  "CM_REVISION",
									Value: cmResourceVersion,
								},
								{
									Name: "KUBERNETES_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace"}},
								},
								selfSignedCertEnv,
								gitSelfSignedCertEnv,
								gitSelfSignedCertHostEnv,
							}},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	cheMultiUser := GetCheMultiUser(checluster)
	if cheMultiUser == "true" {
		chePostgresSecret := checluster.Spec.Database.ChePostgresSecret
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
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: DefaultCheVolumeClaimName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: DefaultCheVolumeClaimName,
					},
				},
			}}
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: DefaultCheVolumeMountPath,
				Name:      DefaultCheVolumeClaimName,
			}}
	}

	// configure readiness probe if debug isn't set
	cheDebug := util.GetValue(checluster.Spec.Server.CheDebug, DefaultCheDebug)
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
	}

	if !isOpenShift {
		runAsUser, err := strconv.ParseInt(util.GetValue(checluster.Spec.K8s.SecurityContextRunAsUser, DefaultSecurityContextRunAsUser), 10, 64)
		if err != nil {
			return nil, err
		}
		fsGroup, err := strconv.ParseInt(util.GetValue(checluster.Spec.K8s.SecurityContextFsGroup, DefaultSecurityContextFsGroup), 10, 64)
		if err != nil {
			return nil, err
		}
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &fsGroup,
		}
	}

	if !util.IsTestMode() {
		err = controllerutil.SetControllerReference(checluster, deployment, clusterAPI.Scheme)
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
		cheServerImageTag := util.GetValue(checluster.Spec.Server.CheImageTag, DefaultCheVersion())
		return checluster.Spec.Server.CheImage + ":" + cheServerImageTag
	}

	defaultCheServerImage := DefaultCheServerImage(checluster)
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
