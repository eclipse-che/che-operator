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
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//go:embed application.yml
var applicationConfig string

func (r *OpenVSXServerReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := r.getDeploymentSpec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get deployment spec: %w", err)
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}

func (r *OpenVSXServerReconciler) getDeploymentSpec(ctx *chetypes.DeployContext) (*appsv1.Deployment, error) {
	cmRevision, err := r.getConfigMapRevision(ctx)
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
									Name:      "config",
									MountPath: "/home/openvsx/server/config",
									ReadOnly:  true,
								},
								{
									Name:      "ca-certs",
									MountPath: "/public-certs",
									ReadOnly:  true,
								},
								{
									Name:      "extensions-data",
									MountPath: "/tmp/extensions",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "CM_REVISION",
									Value: cmRevision,
								},
								{
									Name: "DB_USERNAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
											Key:                  "database-user",
										},
									},
								},
								{
									Name: "DB_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
											Key:                  "database-password",
										},
									},
								},
								envFromSecret("PGDATABASE", secretName, "db-name"),
								envFromSecret("OPENVSX_USER_PAT", secretName, "publisher-token"),
								envFromSecret("OPENVSX_ADMIN_PAT", secretName, "admin-token"),
								{ // TODO
									Name:  "OVSX_REGISTRY_URL",
									Value: ctx.CheCluster.Status.OpenVSXURL,
								},
								{
									Name:  "NODE_EXTRA_CA_CERTS",
									Value: "/public-certs/tls-ca-bundle.pem",
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/actuator/health",
										Port: intstr.FromInt32(serverPort),
									},
								},
								InitialDelaySeconds: 15,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/actuator/health",
										Port: intstr.FromInt32(serverPort),
									},
								},
								InitialDelaySeconds: 60,
								PeriodSeconds:       20,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
							},
						},
						{
							Name: "ca-certs",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: tls.CheMergedCABundleCertsCMName,
									},
								},
							},
						},
						{
							Name: "extensions-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: serverPVCName,
								},
							},
						},
					},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
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
