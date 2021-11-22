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

package dashboard

import (
	"context"
	"fmt"
	"strconv"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	"github.com/eclipse-che/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const CHE_SELF_SIGNED_MOUNT_PATH = "/public-certs/che-self-signed"
const CHE_CUSTOM_CERTS_MOUNT_PATH = "/public-certs/custom"

func (d *Dashboard) getDashboardDeploymentSpec() (*appsv1.Deployment, error) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	var envVars []corev1.EnvVar

	volumes, volumeMounts = d.provisionCustomPublicCA(volumes, volumeMounts)

	selfSignedCertSecretExist, err := tls.IsSelfSignedCASecretExists(d.deployContext)
	if err != nil {
		return nil, err
	}
	if selfSignedCertSecretExist {
		volumes, volumeMounts = d.provisionCheSelfSignedCA(volumes, volumeMounts)
	}

	envVars = append(envVars,
		// todo handle HTTP_PROXY related env vars
		// CHE_HOST is here for backward compatibility. Replaced with CHE_URL
		corev1.EnvVar{
			Name:  "CHE_HOST",
			Value: util.GetCheURL(d.deployContext.CheCluster)},
		corev1.EnvVar{
			Name:  "CHE_URL",
			Value: util.GetCheURL(d.deployContext.CheCluster)},
	)

	if d.deployContext.CheCluster.IsInternalClusterSVCNamesEnabled() {
		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "CHE_INTERNAL_URL",
				Value: fmt.Sprintf("http://%s.%s.svc:8080/api", deploy.CheServiceName, d.deployContext.CheCluster.Namespace)},
		)
	}

	if util.IsOpenShift {
		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "OPENSHIFT_CONSOLE_URL",
				Value: d.evaluateOpenShiftConsoleURL()})
	}

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
					ServiceAccountName: DashboardSA,
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
							Env:          envVars,
							VolumeMounts: volumeMounts,
						},
					},
					Volumes:                       volumes,
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

func (d *Dashboard) evaluateOpenShiftConsoleURL() string {
	console := &configv1.Console{}

	err := d.deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{
		Name:      "cluster",
		Namespace: "openshift-console",
	}, console)

	if err != nil {
		// if error happen don't fail deployment but try again on the next reconcile loop
		log.Error(err, "failed to get OpenShift Console Custom Resource to evaluate URL")
		return ""
	}
	return console.Status.ConsoleURL
}

func (d *Dashboard) provisionCheSelfSignedCA(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
	cheSelfSigned := corev1.Volume{
		Name: "che-self-signed-ca",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: deploy.CheTLSSelfSignedCertificateSecretName,
				Items: []corev1.KeyToPath{
					{
						Key:  "ca.crt",
						Path: "ca.crt",
					},
				},
			},
		},
	}
	volumes = append(volumes, cheSelfSigned)
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      cheSelfSigned.Name,
		MountPath: CHE_SELF_SIGNED_MOUNT_PATH,
	})
	return volumes, volumeMounts
}

func (d *Dashboard) provisionCustomPublicCA(volumes []corev1.Volume, volumeMounts []corev1.VolumeMount) ([]corev1.Volume, []corev1.VolumeMount) {
	customPublicCertsVolume := corev1.Volume{
		Name: "che-custom-ca",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tls.CheAllCACertsConfigMapName,
				},
			},
		},
	}
	volumes = append(volumes, customPublicCertsVolume)
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      customPublicCertsVolume.Name,
		MountPath: CHE_CUSTOM_CERTS_MOUNT_PATH,
	})
	return volumes, volumeMounts
}
