//
// Copyright (c) 2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package dashboard

import (
	"strconv"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (d *Dashboard) getDashboardDeploymentSpec() (*appsv1.Deployment, error) {
	terminationGracePeriodSeconds := int64(30)
	labels, labelsSelector := deploy.GetLabelsAndSelector(d.deployContext.CheCluster, d.component)

	dashboardImageAndTag := util.GetValue(d.deployContext.CheCluster.Spec.Server.DashboardImage, deploy.DefaultDashboardImage(d.deployContext.CheCluster))
	pullPolicy := corev1.PullPolicy(util.GetValue(d.deployContext.CheCluster.Spec.Server.DashboardImagePullPolicy, deploy.DefaultPullPolicyFromDockerImage(dashboardImageAndTag)))

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.component,
			Namespace: d.deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labelsSelector},
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
							Name:            d.component,
							ImagePullPolicy: pullPolicy,
							Image:           dashboardImageAndTag,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										d.deployContext.CheCluster.Spec.Server.DashboardMemoryRequest,
										deploy.DefaultDashboardMemoryRequest),
									corev1.ResourceCPU: util.GetResourceQuantity(
										d.deployContext.CheCluster.Spec.Server.DashboardCpuRequest,
										deploy.DefaultDashboardCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: util.GetResourceQuantity(
										d.deployContext.CheCluster.Spec.Server.DashboardMemoryLimit,
										deploy.DefaultDashboardMemoryLimit),
									corev1.ResourceCPU: util.GetResourceQuantity(
										d.deployContext.CheCluster.Spec.Server.DashboardCpuLimit,
										deploy.DefaultDashboardCpuLimit),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/",
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
										Path: "/",
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
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	if !util.IsOpenShift {
		runAsUser, err := strconv.ParseInt(util.GetValue(d.deployContext.CheCluster.Spec.K8s.SecurityContextRunAsUser, deploy.DefaultSecurityContextRunAsUser), 10, 64)
		if err != nil {
			return nil, err
		}
		fsGroup, err := strconv.ParseInt(util.GetValue(d.deployContext.CheCluster.Spec.K8s.SecurityContextFsGroup, deploy.DefaultSecurityContextFsGroup), 10, 64)
		if err != nil {
			return nil, err
		}
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &runAsUser,
			FSGroup:   &fsGroup,
		}
	}

	return deployment, nil
}
