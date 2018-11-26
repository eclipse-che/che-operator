package operator

import (
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newPostgresDeployment() *appsv1.Deployment {
	name := "postgres"
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres",
			Namespace: namespace,
			Labels:    postgresLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: postgresLabels},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyType("Recreate"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: postgresLabels,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: name + "-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: name + "-data",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1-25",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          name,
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
									Name:      name + "-data",
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
											"psql -h 127.0.0.1 -U " + chePostgresUser + " -q -d " + chePostgresDb + " -c 'SELECT 1'",
										},
									},
								},
								InitialDelaySeconds: 15,
								FailureThreshold:    10,
								SuccessThreshold:    1,
								TimeoutSeconds:      5,
							},
							Env: []corev1.EnvVar{
								{
									Name:  "POSTGRESQL_USER",
									Value: chePostgresUser,
								},
								{
									Name:  "POSTGRESQL_PASSWORD",
									Value: chePostgresPassword,
								},
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
				},
			},
		},
	}
}

// CreatePostgresDeployment creates a deployment with 1 Postgres pod in spec. DB, user and password for Che are created
// via env variables while DB, user  for Keycloak are provisioned in CreatePgJob
func CreatePostgresDeployment() *appsv1.Deployment {
	deployment := newPostgresDeployment()
	if err := sdk.Create(deployment); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create Postgres deployment : %v", err)
		logrus.Error("Operator is exiting")
		os.Exit(1)
	}
	// wait until deployment is scaled to 1 replica to proceed with other deployments
	util.WaitForSuccessfulDeployment(deployment, "Postgres", 40)
	return deployment
}

func newPgJob() *batchv1.Job {
	labels := map[string]string{"app": "pg-job"}
	var backoffLimit int64 = 10
	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pg-job",
			Namespace: namespace,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pg-service-pod",
					Namespace: namespace,
					Labels:    labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "pg-service-pod",
							Image:   "registry.access.redhat.com/rhscl/postgresql-96-rhel7:1-25",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{"/bin/bash"},
							Args: []string{
								"-c",
								"psql \"user=postgres password=" + postgresAdminPassword +
									" host=postgres port=5432\" -c \"CREATE USER keycloak WITH PASSWORD '" +
									keycloakPostgresPassword + "'\" && psql \"user=postgres password=" + postgresAdminPassword +
									" host=postgres port=5432\" -c \"CREATE DATABASE keycloak\" && psql \"user=postgres password=" +
									postgresAdminPassword + " host=postgres port=5432\" -c \"GRANT ALL PRIVILEGES ON DATABASE keycloak TO keycloak\" + " +
									"&& psql \"user=postgres password=" + postgresAdminPassword + " host=postgres port=5432\" -c \"ALTER USER " +
									chePostgresUser + " WITH SUPERUSER\"",
							},
						},
					},
				},
			},
		},
	}
}

// CreatePgJob starts a pod with psql to provision DB for Keycloak and grant SUPERUSER privileges for chePostgresUser
func CreatePgJob() {
	job := newPgJob()
	if err := sdk.Create(job); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create postgres job : %v", err)
		os.Exit(1)
	}
	util.WaitForSuccessfulJobExecution(job, "Postgres", 10)
}
