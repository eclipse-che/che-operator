//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package openvsx_server

import (
	_ "embed"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func (r *OpenVSXServerReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := r.getDeploymentSpec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get deployment spec: %w", err)
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (r *OpenVSXServerReconciler) getDeploymentSpec(ctx *chetypes.DeployContext) (*appsv1.Deployment, error) {
	configRevision, err := r.getConfigRevision(ctx)
	if err != nil {
		return nil, err
	}

	image := defaults.GetOpenVSXImage(ctx.CheCluster)
	imagePullPolicy := utils.GetPullPolicyFromDockerImage(image)

	labels := deploy.GetLabels(constants.OpenVSXServerComponentName)
	credentialsSecretName := openvsx.GetCredentialsSecretName(ctx)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
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
							Name:            constants.OpenVSXServerComponentName,
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									Name:          constants.OpenVSXServerComponentName,
									ContainerPort: constants.OpenVSXServerServicePort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.OpenVSXServerMemoryRequest),
									corev1.ResourceCPU:    resource.MustParse(constants.OpenVSXServerCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.OpenVSXServerMemoryLimit),
									corev1.ResourceCPU:    resource.MustParse(constants.OpenVSXServerCpuLimit),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      constants.OpenVSXServerComponentName + "config",
									MountPath: "/home/openvsx/server/config",
									ReadOnly:  true,
								},
								{
									Name:      constants.OpenVSXServerComponentName,
									MountPath: "/openvsx/extensions", // defined in application.yaml
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/actuator/health",
										Port: intstr.FromInt32(constants.OpenVSXServerServicePort),
									},
								},
								InitialDelaySeconds: 15,
								FailureThreshold:    10,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/actuator/health",
										Port: intstr.FromInt32(constants.OpenVSXServerServicePort),
									},
								},
								InitialDelaySeconds: 30,
								FailureThreshold:    10,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "OVSX_REGISTRY_URL",
									Value: getServiceURL(ctx),
								},
								{
									Name:  "CONFIG_REVISION",
									Value: configRevision,
								},
								utils.EnvVarFromSecret("DB_USERNAME", credentialsSecretName, "database-user"),
								utils.EnvVarFromSecret("DB_PASSWORD", credentialsSecretName, "database-password"),
								utils.EnvVarFromSecret("PGDATABASE", credentialsSecretName, "database-name"),
								utils.EnvVarFromSecret("OPENVSX_USER_PAT", credentialsSecretName, "openvsx-publisher-token"),
								utils.EnvVarFromSecret("OPENVSX_ADMIN_PAT", credentialsSecretName, "openvsx-admin-token"),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: constants.OpenVSXServerComponentName + "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: constants.OpenVSXServerComponentName,
									},
								},
							},
						},
						{
							Name: constants.OpenVSXServerComponentName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: constants.OpenVSXServerComponentName,
								},
							},
						},
					},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: ptr.To(int64(30)),
				},
			},
		},
	}

	deploy.EnsurePodSecurityStandards(deployment, constants.DefaultSecurityContextRunAsUser, constants.DefaultSecurityContextFsGroup)

	if ctx.CheCluster.Spec.Components.OpenVSXRegistry.Server != nil {
		if err := deploy.OverrideDeployment(ctx, deployment, ctx.CheCluster.Spec.Components.OpenVSXRegistry.Server.Deployment); err != nil {
			return nil, fmt.Errorf("failed to override deployment: %w", err)
		}
	}

	return deployment, nil
}
