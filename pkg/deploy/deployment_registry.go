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
package deploy

import (
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	PluginRegistryDeploymentName  = "plugin-registry"
	DevfileRegistryDeploymentName = "devfile-registry"
)

func SyncPluginRegistryDeploymentToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) DeploymentProvisioningStatus {
	registryType := "plugin"
	registryImage := util.GetValue(checluster.Spec.Server.PluginRegistryImage, DefaultPluginRegistryImage(checluster))
	registryImagePullPolicy := corev1.PullPolicy(util.GetValue(string(checluster.Spec.Server.PluginRegistryPullPolicy), DefaultPullPolicyFromDockerImage(registryImage)))
	registryMemoryLimit := util.GetValue(string(checluster.Spec.Server.PluginRegistryMemoryLimit), DefaultPluginRegistryMemoryLimit)
	registryMemoryRequest := util.GetValue(string(checluster.Spec.Server.PluginRegistryMemoryRequest), DefaultPluginRegistryMemoryRequest)
	probePath := "/v3/plugins/"
	pluginImagesEnv := util.GetEnvByRegExp("^.*plugin_registry_image.*$")

	clusterDeployment, err := getClusterDeployment(PluginRegistryDeploymentName, checluster.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := getSpecRegistryDeployment(
		checluster,
		registryType,
		registryImage,
		pluginImagesEnv,
		registryImagePullPolicy,
		registryMemoryLimit,
		registryMemoryRequest,
		probePath,
		clusterAPI)

	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	return SyncDeploymentToCluster(checluster, specDeployment, clusterDeployment, nil, nil, clusterAPI)
}

func SyncDevfileRegistryDeploymentToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) DeploymentProvisioningStatus {
	registryType := "devfile"
	registryImage := util.GetValue(checluster.Spec.Server.DevfileRegistryImage, DefaultDevfileRegistryImage(checluster))
	registryImagePullPolicy := corev1.PullPolicy(util.GetValue(string(checluster.Spec.Server.PluginRegistryPullPolicy), DefaultPullPolicyFromDockerImage(registryImage)))
	registryMemoryLimit := util.GetValue(string(checluster.Spec.Server.DevfileRegistryMemoryLimit), DefaultDevfileRegistryMemoryLimit)
	registryMemoryRequest := util.GetValue(string(checluster.Spec.Server.DevfileRegistryMemoryRequest), DefaultDevfileRegistryMemoryRequest)
	probePath := "/devfiles/"
	devfileImagesEnv := util.GetEnvByRegExp("^.*devfile_registry_image.*$")

	clusterDeployment, err := getClusterDeployment(DevfileRegistryDeploymentName, checluster.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	specDeployment, err := getSpecRegistryDeployment(
		checluster,
		registryType,
		registryImage,
		devfileImagesEnv,
		registryImagePullPolicy,
		registryMemoryLimit,
		registryMemoryRequest,
		probePath,
		clusterAPI)

	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	return SyncDeploymentToCluster(checluster, specDeployment, clusterDeployment, nil, nil, clusterAPI)
}

func getSpecRegistryDeployment(
	checluster *orgv1.CheCluster,
	registryType string,
	registryImage string,
	env []corev1.EnvVar,
	registryImagePullPolicy corev1.PullPolicy,
	registryMemoryLimit string,
	registryMemoryRequest string,
	probePath string,
	clusterAPI ClusterAPI) (*appsv1.Deployment, error) {

	terminationGracePeriodSeconds := int64(30)
	name := registryType + "-registry"
	labels := GetLabels(checluster, name)
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
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &_1,
			RevisionHistoryLimit: &_2,
			Selector:             &metav1.LabelSelector{MatchLabels: labels},
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
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(registryMemoryRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(registryMemoryLimit),
								},
							},
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
								PeriodSeconds:       10,
								SuccessThreshold:    1,
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
						},
					},
					RestartPolicy:                 "Always",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		},
	}

	if !util.IsTestMode() {
		err := controllerutil.SetControllerReference(checluster, deployment, clusterAPI.Scheme)
		if err != nil {
			return nil, err
		}
	}

	return deployment, nil
}
