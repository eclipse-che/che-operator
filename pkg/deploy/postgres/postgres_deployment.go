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
package postgres

import (
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	PostgresVersion9_6  = "9.6"
	PostgresVersion13_3 = "13.3"
)

var (
	postgresAdminPassword = utils.GeneratePassword(12)
)

func (p *PostgresReconciler) getDeploymentSpec(clusterDeployment *appsv1.Deployment, ctx *chetypes.DeployContext) (*appsv1.Deployment, error) {
	terminationGracePeriodSeconds := int64(30)
	labels, labelSelector := deploy.GetLabelsAndSelector(constants.PostgresName)
	chePostgresDb := utils.GetValue(ctx.CheCluster.Spec.Components.Database.PostgresDb, constants.DefaultPostgresDb)
	postgresImage, err := getPostgresImage(clusterDeployment, ctx.CheCluster)
	if err != nil {
		return nil, err
	}
	pullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(postgresImage))

	if clusterDeployment != nil {
		clusterContainer := &clusterDeployment.Spec.Template.Spec.Containers[0]
		value := utils.GetEnv(clusterContainer.Env, "POSTGRESQL_ADMIN_PASSWORD")
		if value != "" {
			postgresAdminPassword = value
		}
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.PostgresName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labelSelector},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyType("Recreate"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: constants.DefaultPostgresVolumeClaimName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: constants.DefaultPostgresVolumeClaimName,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            constants.PostgresName,
							Image:           postgresImage,
							ImagePullPolicy: pullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          constants.PostgresName,
									ContainerPort: 5432,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.DefaultPostgresMemoryRequest),
									corev1.ResourceCPU:    resource.MustParse(constants.DefaultPostgresCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.DefaultPostgresMemoryLimit),
									corev1.ResourceCPU:    resource.MustParse(constants.DefaultPostgresCpuLimit),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      constants.DefaultPostgresVolumeClaimName,
									MountPath: "/var/lib/pgsql/data",
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{
											"/bin/sh",
											"-i",
											"-c",
											"psql -h 127.0.0.1 -U $POSTGRESQL_USER -q -d " + chePostgresDb + " -c 'SELECT 1'",
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
								Handler: corev1.Handler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(5432),
									},
								},
								InitialDelaySeconds: 30,
								FailureThreshold:    10,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "POSTGRESQL_DATABASE",
									Value: chePostgresDb,
								},
								{
									Name:  "POSTGRESQL_ADMIN_PASSWORD",
									Value: postgresAdminPassword,
								},
							}},
					},
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					RestartPolicy:                 "Always",
				},
			},
		},
	}

	container := &deployment.Spec.Template.Spec.Containers[0]

	chePostgresCredentialsSecret := utils.GetValue(ctx.CheCluster.Spec.Components.Database.CredentialsSecretName, constants.DefaultPostgresCredentialsSecret)
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name: "POSTGRESQL_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "user",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: chePostgresCredentialsSecret,
					},
				},
			},
		}, corev1.EnvVar{
			Name: "POSTGRESQL_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: "password",
					LocalObjectReference: corev1.LocalObjectReference{
						Name: chePostgresCredentialsSecret,
					},
				},
			},
		})

	if !infrastructure.IsOpenShift() {
		var runAsUser int64 = 26
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &runAsUser,
		}
	}

	deploy.CustomizeDeployment(deployment, ctx.CheCluster.Spec.Components.Database.Deployment, false)
	return deployment, nil
}

func getPostgresImage(clusterDeployment *appsv1.Deployment, cheCluster *chev2.CheCluster) (string, error) {
	if cheCluster.Spec.Components.Database.Deployment != nil &&
		len(cheCluster.Spec.Components.Database.Deployment.Containers) > 0 &&
		cheCluster.Spec.Components.Database.Deployment.Containers[0].Image != "" {
		// use image explicitly set in a CR
		return cheCluster.Spec.Components.Database.Deployment.Containers[0].Image, nil
	} else if cheCluster.Status.PostgresVersion == PostgresVersion9_6 {
		return defaults.GetPostgresImage(cheCluster), nil
	} else if strings.HasPrefix(cheCluster.Status.PostgresVersion, "13.") {
		return defaults.GetPostgres13Image(cheCluster), nil
	} else if cheCluster.Status.PostgresVersion == "" {
		if clusterDeployment == nil {
			// Use PostgreSQL 13.3 for a new deployment if there is so.
			// It allows to work in downstream until a new image is ready for production.
			postgres13Image := defaults.GetPostgres13Image(cheCluster)
			if postgres13Image != "" {
				return postgres13Image, nil
			} else {
				return defaults.GetPostgresImage(cheCluster), nil
			}
		} else {
			// Keep using current image
			return clusterDeployment.Spec.Template.Spec.Containers[0].Image, nil
		}
	}

	return "", fmt.Errorf("PostgreSQL image for '%s' version not found", cheCluster.Status.PostgresVersion)
}
