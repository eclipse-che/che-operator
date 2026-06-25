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

package database

import (
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

func (p *OpenVSXDatabaseReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := getDeploymentSpec(ctx)
	if err != nil {
		return false, fmt.Errorf("unable to get deployment spec: %w", err)
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func getDeploymentSpec(ctx *chetypes.DeployContext) (*appsv1.Deployment, error) {
	image := defaults.GetOpenVSXDatabaseImage(ctx.CheCluster)
	imagePullPolicy := utils.GetPullPolicyFromDockerImage(image)

	labels := deploy.GetLabels(constants.OpenVSXDatabaseComponentName)
	credentialsSecretName := openvsx.GetCredentialsSecretName(ctx)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXDatabaseComponentName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            constants.OpenVSXDatabaseComponentName,
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							Ports: []corev1.ContainerPort{
								{
									Name:          constants.OpenVSXDatabaseComponentName,
									ContainerPort: constants.OpenVSXDatabaseServicePort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.OpenVSXDatabaseMemoryRequest),
									corev1.ResourceCPU:    resource.MustParse(constants.OpenVSXDatabaseCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.OpenVSXDatabaseMemoryLimit),
									corev1.ResourceCPU:    resource.MustParse(constants.OpenVSXDatabaseCpuLimit),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      constants.OpenVSXDatabaseComponentName,
									MountPath: "/var/lib/pgsql/data",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh",
											"-i",
											"-c",
											"psql -h 127.0.0.1 -U $POSTGRESQL_USER -q -d $POSTGRESQL_DATABASE -c 'SELECT 1'",
										},
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
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt32(constants.OpenVSXDatabaseServicePort),
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
									Name: "POSTGRESQL_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
											Key:                  "database-user",
										},
									},
								},
								{
									Name: "POSTGRESQL_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
											Key:                  "database-password",
										},
									},
								},
								{
									Name: "POSTGRESQL_DATABASE",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: credentialsSecretName},
											Key:                  "database-name",
										},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: constants.OpenVSXDatabaseComponentName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: constants.OpenVSXDatabaseComponentName,
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

	// `26` is a default user and group id in the container file
	kubernetesUserId := int64(26)
	kubernetesGroupId := int64(26)

	deploy.EnsurePodSecurityStandards(deployment, kubernetesUserId, kubernetesGroupId)

	if ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database != nil {
		if err := deploy.OverrideDeployment(ctx, deployment, ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database.Deployment); err != nil {
			return nil, fmt.Errorf("failed to override deployment: %w", err)
		}
	}

	return deployment, nil
}
