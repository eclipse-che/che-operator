//
// Copyright (c) 2012-2021 Red Hat, Inc.
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
	v12 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func GetSpecRegistryDeployment(
	deployContext *deploy.DeployContext,
	registryType string,
	registryImage string,
	env []v1.EnvVar,
	registryImagePullPolicy v1.PullPolicy,
	resources v1.ResourceRequirements,
	probePath string) *v12.Deployment {

	terminationGracePeriodSeconds := int64(30)
	name := registryType + "-registry"
	labels, labelSelector := deploy.GetLabelsAndSelector(deployContext.CheCluster, name)
	_25Percent := intstr.FromString("25%")
	_1 := int32(1)
	_2 := int32(2)
	isOptional := true
	deployment := &v12.Deployment{
		TypeMeta: v13.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: v13.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: v12.DeploymentSpec{
			Replicas:             &_1,
			RevisionHistoryLimit: &_2,
			Selector:             &v13.LabelSelector{MatchLabels: labelSelector},
			Strategy: v12.DeploymentStrategy{
				Type: v12.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &v12.RollingUpdateDeployment{
					MaxSurge:       &_25Percent,
					MaxUnavailable: &_25Percent,
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: v13.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            "che-" + name,
							Image:           registryImage,
							ImagePullPolicy: registryImagePullPolicy,
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
							},
							Env: env,
							EnvFrom: []v1.EnvFromSource{
								{
									ConfigMapRef: &v1.ConfigMapEnvSource{
										Optional: &isOptional,
										LocalObjectReference: v1.LocalObjectReference{
											Name: registryType + "-registry",
										},
									},
								},
							},
							Resources: resources,
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/" + registryType + "s/",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: v1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 3,
								FailureThreshold:    10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/" + registryType + "s/",
										Port: intstr.IntOrString{
											Type:   intstr.Int,
											IntVal: int32(8080),
										},
										Scheme: v1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 30,
								FailureThreshold:    10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								PeriodSeconds:       10,
							},
							SecurityContext: &v1.SecurityContext{
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{"ALL"},
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
