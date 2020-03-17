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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewCheDeployment(cr *orgv1.CheCluster, cheImageAndTag string, cmRevision string, isOpenshift bool) (*appsv1.Deployment, error) {
	labels := GetLabels(cr, util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor))
	optionalEnv := true
	cheFlavor := util.GetValue(cr.Spec.Server.CheFlavor, DefaultCheFlavor)
	memRequest := util.GetValue(cr.Spec.Server.ServerMemoryRequest, DefaultServerMemoryRequest)
	selfSignedCertEnv := corev1.EnvVar{
		Name:  "CHE_SELF__SIGNED__CERT",
		Value: "",
	}
	customPublicCertsVolumeSource := corev1.VolumeSource{}
	if cr.Spec.Server.ServerTrustStoreConfigMapName != "" {
		customPublicCertsVolumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cr.Spec.Server.ServerTrustStoreConfigMapName,
				},
			},
		}
	}
	customPublicCertsVolume := corev1.Volume{
		Name: "che-public-certs",
		VolumeSource: customPublicCertsVolumeSource,
	}
	customPublicCertsVolumeMount := corev1.VolumeMount{
		Name: "che-public-certs",
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
	if cr.Spec.Server.SelfSignedCert {
		selfSignedCertEnv = corev1.EnvVar{
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
		}
	}
	if cr.Spec.Server.GitSelfSignedCert {
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

	memLimit := util.GetValue(cr.Spec.Server.ServerMemoryLimit, DefaultServerMemoryLimit)
	pullPolicy := corev1.PullPolicy(util.GetValue(string(cr.Spec.Server.CheImagePullPolicy), DefaultPullPolicyFromDockerImage(cheImageAndTag)))

	cheDeployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheFlavor,
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
					ServiceAccountName: "che",
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
									Value: cmRevision,
								},
								{
									Name: "KUBERNETES_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace"}},
								},
								selfSignedCertEnv,
								gitSelfSignedCertEnv,
								gitSelfSignedCertHostEnv,
							}},
					},
				},
			},
		},
	}

	cheMultiUser := GetCheMultiUser(cr)
	if cheMultiUser == "false" {
		cheDeployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: DefaultCheVolumeClaimName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: DefaultCheVolumeClaimName,
					},
				},
			}}
		cheDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				MountPath: DefaultCheVolumeMountPath,
				Name:      DefaultCheVolumeClaimName,
			}}
	}

	// configure readiness probe if debug isn't set
	cheDebug := util.GetValue(cr.Spec.Server.CheDebug, DefaultCheDebug)
	if cheDebug != "true" {
		cheDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
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
		}
	}

	if !isOpenshift {
		runAsUser, err := strconv.ParseInt(util.GetValue(cr.Spec.K8s.SecurityContextRunAsUser, DefaultSecurityContextRunAsUser), 10, 64)
		if err != nil {
			return nil, err
		}
		fsGroup, err := strconv.ParseInt(util.GetValue(cr.Spec.K8s.SecurityContextFsGroup, DefaultSecurityContextFsGroup), 10, 64)
		if err != nil {
			return nil, err
		}
		cheDeployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &fsGroup,
		}
	}

	return &cheDeployment, nil
}
