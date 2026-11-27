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

package dashboard

import (
	"fmt"
	"maps"
	"slices"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	selfSignedCertVolumeName = "che-self-signed-ca"
	selfSignedCertMountPath  = "/public-certs/che-self-signed"

	customCertsVolumeName = "che-custom-ca"
	customCertsMountPath  = "/public-certs/custom"

	memoryLimit   = "1024Mi"
	memoryRequest = "32Mi"
	cpuLimit      = "500m"
	cpuRequest    = "100m"
)

func (r *DashboardReconciler) getDashboardDeploymentSpec(cheCtx *chetypes.CheContext) (*appsv1.Deployment, error) {
	image := defaults.GetDashboardImage(cheCtx.CheCluster)
	imagePullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))

	envVars, err := r.collectEnvVar(cheCtx)
	if err != nil {
		return nil, err
	}

	volumes, volumeMounts, err := r.collectVolumes(cheCtx)
	if err != nil {
		return nil, err
	}

	selector := chelabels.GetLegacySelector(getComponentName())
	labels := chelabels.GetStandardLabels(getComponentName())
	maps.Copy(labels, selector)

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(),
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: getComponentName(),
					Containers: []corev1.Container{
						{
							Name:            getComponentName(),
							ImagePullPolicy: imagePullPolicy,
							Image:           image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memoryRequest),
									corev1.ResourceCPU:    resource.MustParse(cpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(memoryLimit),
									corev1.ResourceCPU:    resource.MustParse(cpuLimit),
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
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
								ProbeHandler: corev1.ProbeHandler{
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
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: new(int64(30)),
				},
			},
		},
	}

	if err := resources.Deployment.TrimCPULimitIfNoLimitRange(cheCtx, deployment); err != nil {
		return nil, err
	}

	pod := &deployment.Spec.Template.Spec
	resources.Pod.EnsureSecurityStandards(pod, constants.DefaultSecurityContextRunAsUser, constants.DefaultSecurityContextFsGroup)

	overrides := cheCtx.CheCluster.Spec.Components.Dashboard.Deployment
	resources.Deployment.ApplyOverrides(deployment, overrides)

	return deployment, nil
}

func (r *DashboardReconciler) collectVolumes(cheCtx *chetypes.CheContext) ([]corev1.Volume, []corev1.VolumeMount, error) {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	volumes = append(volumes,
		corev1.Volume{
			Name:         customCertsVolumeName,
			VolumeSource: resources.VolumeSource.FromConfigMap(tls.CheMergedCABundleCertsCMName),
		},
	)

	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      customCertsVolumeName,
			MountPath: customCertsMountPath,
		},
	)

	exists, err := resources.Secret.Get(cheCtx, constants.SelfSignedCertificateSecretName, cheCtx.CheCluster.Namespace, &corev1.Secret{})
	if err != nil {
		return nil, nil, err
	}
	if exists {
		volumes = append(volumes,
			corev1.Volume{
				Name:         selfSignedCertVolumeName,
				VolumeSource: resources.VolumeSource.FromSecret(constants.SelfSignedCertificateSecretName, "ca.crt"),
			})

		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      selfSignedCertVolumeName,
				MountPath: selfSignedCertMountPath,
			},
		)
	}

	return volumes, volumeMounts, nil
}

func (r *DashboardReconciler) collectEnvVar(cheCtx *chetypes.CheContext) ([]corev1.EnvVar, error) {
	envVars := utils.GetGetArchitectureDependentEnvsByRegExp("^RELATED_IMAGE_sample_.*$")
	envVars = append(envVars, cheCtx.Proxy.AsEnvVars()...)
	envVars = append(envVars, utils.GetEnvsByRegExp("^CHE_DEFAULT_SPEC.*")...)
	envVars = append(envVars,
		// CHE_HOST is here for backward compatibility. Replaced with CHE_URL
		corev1.EnvVar{
			Name:  "CHE_HOST",
			Value: "https://" + cheCtx.CheHost},
		corev1.EnvVar{
			Name:  "CHE_URL",
			Value: "https://" + cheCtx.CheHost},
		corev1.EnvVar{
			Name:  "CHECLUSTER_CR_NAMESPACE",
			Value: cheCtx.CheCluster.Namespace},
		corev1.EnvVar{
			Name:  "CHECLUSTER_CR_NAME",
			Value: cheCtx.CheCluster.Name},
		corev1.EnvVar{
			Name:  "DWO_NAMESPACE",
			Value: cheCtx.DWONamespace,
		},
		corev1.EnvVar{
			Name:  "CHE_DASHBOARD_INTERNAL_URL",
			Value: fmt.Sprintf("http://%s.%s.svc:8080", getComponentName(), cheCtx.CheCluster.Namespace),
		},
		corev1.EnvVar{
			Name:  "CHE_INTERNAL_URL",
			Value: fmt.Sprintf("http://%s.%s.svc:8080/api", deploy.CheServiceName, cheCtx.CheCluster.Namespace),
		},
	)

	if !cheCtx.CheCluster.IsInternalPluginRegistryDisabled() {
		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL",
				Value: fmt.Sprintf("http://%s.%s.svc:8080/v3", constants.PluginRegistryName, cheCtx.CheCluster.Namespace),
			},
		)
	}

	if cheCtx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		envVars = slices.DeleteFunc(
			envVars,
			func(envVar corev1.EnvVar) bool {
				return envVar.Name == "CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL"
			},
		)

		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL",
				Value: cheCtx.CheCluster.Status.OpenVSXURL, // only public url
			},
		)
	}

	if infrastructure.IsOpenShift() {
		console := &configv1.Console{}
		exists, err := resources.Console.Get(cheCtx, console)
		if err != nil {
			return nil, err
		}

		if exists {
			envVars = append(envVars,
				corev1.EnvVar{
					Name:  "OPENSHIFT_CONSOLE_URL",
					Value: console.Status.ConsoleURL,
				},
			)
		} else {
			logger.Info("OpenShift console not found")
		}
	}

	return envVars, nil
}
