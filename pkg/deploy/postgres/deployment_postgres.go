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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	postgresAdminPassword = util.GeneratePasswd(12)
)

func SyncPostgresDeploymentToCluster(deployContext *deploy.DeployContext) deploy.DeploymentProvisioningStatus {
	clusterDeployment, err := deploy.GetClusterDeployment(deploy.PostgresName, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := getSpecPostgresDeployment(deployContext, clusterDeployment)
	if err != nil {
		return deploy.DeploymentProvisioningStatus{
			ProvisioningStatus: deploy.ProvisioningStatus{Err: err},
		}
	}

	return deploy.SyncDeploymentToCluster(deployContext, specDeployment, clusterDeployment, nil, nil)
}

func getSpecPostgresDeployment(deployContext *deploy.DeployContext, clusterDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	isOpenShift, _, err := util.DetectOpenShift()
	if err != nil {
		return nil, err
	}

	terminationGracePeriodSeconds := int64(30)
	labels, labelSelector := deploy.GetDeploymentLabelsAndSelector(deployContext.CheCluster, deploy.PostgresName)
	chePostgresDb := util.GetValue(deployContext.CheCluster.Spec.Database.ChePostgresDb, "dbche")
	postgresImage := util.GetValue(deployContext.CheCluster.Spec.Database.PostgresImage, deploy.DefaultPostgresImage(deployContext.CheCluster))
	pullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Database.PostgresImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(postgresImage)))

	if clusterDeployment != nil {
		env := clusterDeployment.Spec.Template.Spec.Containers[0].Env
		for _, e := range env {
			if "POSTGRESQL_ADMIN_PASSWORD" == e.Name {
				postgresAdminPassword = e.Value
				break
			}
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
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("1Gi"),
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

	chePostgresSecret := deployContext.CheCluster.Spec.Database.ChePostgresSecret
	if len(chePostgresSecret) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env,
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
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env,
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
