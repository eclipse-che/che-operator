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
package registry

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetSpecRegistryDeployment(
	deployContext *deploy.DeployContext,
	registryType string,
	registryImage string,
	env []corev1.EnvVar,
	registryImagePullPolicy corev1.PullPolicy,
	resources corev1.ResourceRequirements,
	probePath string) *appsv1.Deployment {

	terminationGracePeriodSeconds := int64(30)
	name := registryType + "-registry"
	labels, labelSelector := deploy.GetLabelsAndSelector(deployContext.CheCluster, name)
	_25Percent := intstr.FromString("25%")
	_1 := int32(1)
	_2 := int32(2)
	isOptional := true
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &_1,
			RevisionHistoryLimit: &_2,
			Selector:             &metav1.LabelSelector{MatchLabels: labelSelector},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &_25Percent,
					MaxUnavailable: &_25Percent,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "che-" + name,
							Image:           registryImage,
							ImagePullPolicy: registryImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
							},
							Env: env,
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										Optional: &isOptional,
										LocalObjectReference: corev1.LocalObjectReference{
											Name: registryType + "-registry",
										},
									},
								},
							},
							Resources: resources,
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/" + registryType + "s/",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 3,
								FailureThreshold:    10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/" + registryType + "s/",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 30,
								FailureThreshold:    10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	return deployment
}
