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
	"fmt"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	identity_provider "github.com/eclipse-che/che-operator/pkg/deploy/identity-provider"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	postgresAdminPassword = util.GeneratePasswd(12)
)

type Postgres struct {
	deployContext *deploy.DeployContext
	isMultiUser   bool
}

func NewPostgres(deployContext *deploy.DeployContext) *Postgres {
	return &Postgres{
		deployContext: deployContext,
		isMultiUser:   deploy.GetCheMultiUser(deployContext.CheCluster) == "true",
	}
}

func (p *Postgres) Sync() (bool, error) {
	if p.deployContext.CheCluster.Spec.Database.ExternalDb {
		return true, nil
	}

	done, err := p.syncService()
	if !done {
		return false, err
	}

	done, err = p.syncPVC()
	if !done {
		return false, err
	}

	done, err = p.syncDeployment()
	if !done {
		return false, err
	}

	if !p.deployContext.CheCluster.Status.DbProvisoned {
		if !util.IsTestMode() { // ignore in tests
			done, err = p.provisionDB()
			if !done {
				return false, err
			}
		}
	}

	return true, nil
}

func (p *Postgres) syncService() (bool, error) {
	if !p.isMultiUser {
		return deploy.DeleteNamespacedObject(p.deployContext, deploy.PostgresName, &corev1.Service{})
	}
	return deploy.SyncServiceToCluster(p.deployContext, deploy.PostgresName, []string{deploy.PostgresName}, []int32{5432}, deploy.PostgresName)
}

func (p *Postgres) syncPVC() (bool, error) {
	if !p.isMultiUser {
		return deploy.DeleteNamespacedObject(p.deployContext, deploy.DefaultPostgresVolumeClaimName, &corev1.PersistentVolumeClaim{})
	}

	done, err := deploy.SyncPVCToCluster(p.deployContext, deploy.DefaultPostgresVolumeClaimName, "1Gi", deploy.PostgresName)
	if !done {
		logrus.Infof("Waiting on pvc '%s' to be bound. Sometimes PVC can be bound only when the first consumer is created.", deploy.DefaultPostgresVolumeClaimName)
	}
	return done, err
}

func (p *Postgres) syncDeployment() (bool, error) {
	if !p.isMultiUser {
		return deploy.DeleteNamespacedObject(p.deployContext, deploy.PostgresName, &appsv1.Deployment{})
	}

	clusterDeployment := &appsv1.Deployment{}
	exists, err := deploy.GetNamespacedObject(p.deployContext, deploy.PostgresName, clusterDeployment)
	if err != nil {
		return false, err
	}

	if !exists {
		clusterDeployment = nil
	}

	specDeployment, err := p.getDeploymentSpec(clusterDeployment)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(p.deployContext, specDeployment, deploy.DefaultDeploymentDiffOpts)
}

func (p *Postgres) provisionDB() (bool, error) {
	identityProviderPostgresPassword := p.deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresPassword
	identityProviderPostgresSecret := p.deployContext.CheCluster.Spec.Auth.IdentityProviderPostgresSecret
	if identityProviderPostgresSecret != "" {
		secret := &corev1.Secret{}
		exists, err := deploy.GetNamespacedObject(p.deployContext, identityProviderPostgresSecret, secret)
		if err != nil {
			return false, err
		} else if !exists {
			return false, fmt.Errorf("Secret '%s' not found", identityProviderPostgresSecret)
		}
		identityProviderPostgresPassword = string(secret.Data["password"])
	}

	_, err := util.K8sclient.ExecIntoPod(
		p.deployContext.CheCluster,
		deploy.PostgresName,
		func(cr *orgv1.CheCluster) (string, error) {
			return identity_provider.GetPostgresProvisionCommand(identityProviderPostgresPassword), nil
		},
		"create Keycloak DB, user, privileges")
	if err != nil {
		return false, err
	}

	p.deployContext.CheCluster.Status.DbProvisoned = true
	err = deploy.UpdateCheCRStatus(p.deployContext, "status: provisioned with DB and user", "true")
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p *Postgres) getDeploymentSpec(clusterDeployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	terminationGracePeriodSeconds := int64(30)
	labels, labelSelector := deploy.GetLabelsAndSelector(p.deployContext.CheCluster, deploy.PostgresName)
	chePostgresDb := util.GetValue(p.deployContext.CheCluster.Spec.Database.ChePostgresDb, "dbche")
	postgresImage := util.GetValue(p.deployContext.CheCluster.Spec.Database.PostgresImage, deploy.DefaultPostgresImage(p.deployContext.CheCluster))
	pullPolicy := corev1.PullPolicy(util.GetValue(string(p.deployContext.CheCluster.Spec.Database.PostgresImagePullPolicy), deploy.DefaultPullPolicyFromDockerImage(postgresImage)))

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
			Namespace: p.deployContext.CheCluster.Namespace,
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
										p.deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Requests.Memory,
										deploy.DefaultPostgresMemoryRequest),
									corev1.ResourceCPU: util.GetResourceQuantity(
										p.deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Requests.Cpu,
										deploy.DefaultPostgresCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										p.deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Limits.Memory,
										deploy.DefaultPostgresMemoryLimit),
									corev1.ResourceCPU: util.GetResourceQuantity(
										p.deployContext.CheCluster.Spec.Database.ChePostgresContainerResources.Limits.Cpu,
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

	chePostgresSecret := p.deployContext.CheCluster.Spec.Database.ChePostgresSecret
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
				Value: p.deployContext.CheCluster.Spec.Database.ChePostgresUser,
			}, corev1.EnvVar{
				Name:  "POSTGRESQL_PASSWORD",
				Value: p.deployContext.CheCluster.Spec.Database.ChePostgresPassword,
			})
	}

	if !util.IsOpenShift {
		var runAsUser int64 = 26
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &runAsUser,
		}
	}

	return deployment, nil
}
