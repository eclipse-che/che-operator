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
package postgres

import (
	"github.com/eclipse/che-operator/pkg/deploy"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	postgresAdminPassword = util.GeneratePasswd(12)
)

func SyncPostgresDeploymentToCluster(deployContext *deploy.DeployContext) (bool, error) {
	clusterDeployment, err := deploy.GetClusterDeployment(deploy.PostgresName, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return false, err
	}

	specDeployment, err := GetSpecPostgresDeployment(deployContext, clusterDeployment)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentToCluster(deployContext, specDeployment, clusterDeployment, nil, nil)
}

func GetSpecPostgresDeployment(deployContext *deploy.DeployContext, clusterDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	isOpenShift, _, err := util.DetectOpenShift()
	if err != nil {
		return nil, err
	}

	terminationGracePeriodSeconds := int64(30)
	labels, labelSelector := deploy.GetLabelsAndSelector(deployContext.CheCluster, deploy.PostgresName)
	chePostgresDb := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresDb, "dbche")
	postgresImage := util.GetValue(deployContext.CheCluster.Spec.Database.PostgresImage, deploy.DefaultPostgresImage(deployContext.CheCluster))
	pullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Database.PostgresImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(postgresImage)))

	if clusterDeployment != nil {
		clusterContainer := &clusterDeployment.Spec.Template.Spec.Containers[0]
		env := util.FindEnv(clusterContainer.Env, "POSTGRESQL_ADMIN_PASSWORD")
		if env != nil {
			postgresAdminPassword = env.Value
		}
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.PostgresName,
			Namespace: deployContext.CheCluster.Namespace,
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
							Name: deploy.DefaultPostgresVolumeClaimName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: deploy.DefaultPostgresVolumeClaimName,
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            deploy.PostgresName,
							Image:           postgresImage,
							ImagePullPolicy: pullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          deploy.PostgresName,
									ContainerPort: 5432,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Requests.Memory,
										deploy.DefaultPostgresMemoryRequest),
									corev1.ResourceCPU: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Requests.Cpu,
										deploy.DefaultPostgresCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Limits.Memory,
										deploy.DefaultPostgresMemoryLimit),
									corev1.ResourceCPU: util.GetResourceQuantity(
										deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Limits.Cpu,
										deploy.DefaultPostgresCpuLimit),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      deploy.DefaultPostgresVolumeClaimName,
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

	chePostgresSecret := deployContext.CheCluster.Spec.Database.ChePostgresSecret
	if len(chePostgresSecret) > 0 {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name: "POSTGRESQL_USER",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "user",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: chePostgresSecret,
						},
					},
				},
			}, corev1.EnvVar{
				Name: "POSTGRESQL_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "password",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: chePostgresSecret,
						},
					},
				},
			})
	} else {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  "POSTGRESQL_USER",
				Value: deployContext.CheCluster.Spec.Database.ChePostgresUser,
			}, corev1.EnvVar{
				Name:  "POSTGRESQL_PASSWORD",
				Value: deployContext.CheCluster.Spec.Database.ChePostgresPassword,
			})
	}

	if !isOpenShift {
		var runAsUser int64 = 26
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &runAsUser,
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
