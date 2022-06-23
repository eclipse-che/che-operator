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
package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var DefaultDeploymentDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(appsv1.Deployment{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas", "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
	cmpopts.IgnoreFields(appsv1.DeploymentStrategy{}, "RollingUpdate"),
	cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy", "SecurityContext"),
	cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "SecurityContext", "DeprecatedServiceAccount"),
	cmpopts.IgnoreFields(corev1.ConfigMapVolumeSource{}, "DefaultMode"),
	cmpopts.IgnoreFields(corev1.SecretVolumeSource{}, "DefaultMode"),
	cmpopts.IgnoreFields(corev1.VolumeSource{}, "EmptyDir"),
	cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	}),
}

func SyncDeploymentSpecToCluster(
	deployContext *chetypes.DeployContext,
	deploymentSpec *appsv1.Deployment,
	deploymentDiffOpts cmp.Options) (bool, error) {

	if err := MountSecrets(deploymentSpec, deployContext); err != nil {
		return false, err
	}

	if err := MountConfigMaps(deploymentSpec, deployContext); err != nil {
		return false, err
	}

	done, err := Sync(deployContext, deploymentSpec, deploymentDiffOpts)
	if err != nil || !done {
		// Failed to sync (update), let's delete and create instead
		if err != nil && strings.Contains(err.Error(), "field is immutable") {
			if _, err := DeleteNamespacedObject(deployContext, deploymentSpec.Name, &appsv1.Deployment{}); err != nil {
				return false, err
			}

			// Deleted successfully, return original error
			return false, err
		}
		return false, err
	}

	// always return true for tests
	if test.IsTestMode() {
		return true, nil
	}

	actual := &appsv1.Deployment{}
	exists, err := GetNamespacedObject(deployContext, deploymentSpec.ObjectMeta.Name, actual)
	if !exists || err != nil {
		return false, err
	}

	if actual.Spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType && actual.Status.Replicas > 1 {
		logrus.Infof("Deployment %s is in the rolling update state.", deploymentSpec.Name)
	}

	provisioned := actual.Status.AvailableReplicas == 1 && actual.Status.Replicas == 1
	return provisioned, nil
}

// CustomizeDeployment customize deployment
func CustomizeDeployment(deployment *appsv1.Deployment, customDeployment *chev2.Deployment, useCustomSecurityContext bool) error {
	if customDeployment == nil || len(customDeployment.Containers) == 0 {
		return nil
	}

	for index, _ := range deployment.Spec.Template.Spec.Containers {
		container := &deployment.Spec.Template.Spec.Containers[index]

		customContainer := &customDeployment.Containers[0]
		if len(deployment.Spec.Template.Spec.Containers) != 1 {
			customContainer = getContainerByName(container.Name, customDeployment.Containers)
			if customContainer == nil {
				continue
			}
		}

		container.Image = utils.GetValue(customContainer.Image, container.Image)

		if customContainer.ImagePullPolicy != "" {
			container.ImagePullPolicy = customContainer.ImagePullPolicy
		} else if container.ImagePullPolicy == "" {
			container.ImagePullPolicy = corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(container.Image))
		}

		if customContainer.Resources != nil {
			if customContainer.Resources.Requests != nil {
				if !customContainer.Resources.Requests.Memory.IsZero() {
					container.Resources.Requests[corev1.ResourceMemory] = customContainer.Resources.Requests.Memory
				}
				if !customContainer.Resources.Requests.Cpu.IsZero() {
					container.Resources.Requests[corev1.ResourceCPU] = customContainer.Resources.Requests.Cpu
				}
			}

			if customContainer.Resources.Limits != nil {
				if !customContainer.Resources.Limits.Memory.IsZero() {
					container.Resources.Limits[corev1.ResourceMemory] = customContainer.Resources.Limits.Memory
				}
				if !customContainer.Resources.Limits.Cpu.IsZero() {
					container.Resources.Limits[corev1.ResourceCPU] = customContainer.Resources.Limits.Cpu
				}
			}
		}
	}

	if !infrastructure.IsOpenShift() {
		if useCustomSecurityContext {
			if customDeployment.SecurityContext != nil {
				if customDeployment.SecurityContext.FsGroup != nil {
					deployment.Spec.Template.Spec.SecurityContext.FSGroup = pointer.Int64Ptr(*customDeployment.SecurityContext.FsGroup)
				}

				if customDeployment.SecurityContext.RunAsUser != nil {
					deployment.Spec.Template.Spec.SecurityContext.RunAsUser = pointer.Int64Ptr(*customDeployment.SecurityContext.RunAsUser)
				}
			}
		}
	}

	return nil
}

func getOrDefaultQuantity(value resource.Quantity, defaultValue resource.Quantity) resource.Quantity {
	if !value.IsZero() {
		return value
	}
	return defaultValue
}

func getContainerByName(name string, containers []chev2.Container) *chev2.Container {
	for _, c := range containers {
		if c.Name == name {
			return &c
		}
	}

	return nil
}

// MountSecrets mounts secrets into a container as a file or as environment variable.
// Secrets are selected by the following labels:
// - app.kubernetes.io/part-of=che.eclipse.org
// - app.kubernetes.io/component=<DEPLOYMENT-NAME>-secret
func MountSecrets(specDeployment *appsv1.Deployment, deployContext *chetypes.DeployContext) error {
	secrets := &corev1.SecretList{}

	kubernetesPartOfLabelSelectorRequirement, _ := labels.NewRequirement(constants.KubernetesPartOfLabelKey, selection.Equals, []string{constants.CheEclipseOrg})
	kubernetesComponentLabelSelectorRequirement, _ := labels.NewRequirement(constants.KubernetesComponentLabelKey, selection.Equals, []string{specDeployment.Name + "-secret"})

	listOptions := &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*kubernetesPartOfLabelSelectorRequirement).Add(*kubernetesComponentLabelSelectorRequirement),
	}
	if err := deployContext.ClusterAPI.Client.List(context.TODO(), secrets, listOptions); err != nil {
		return err
	}

	// sort secrets by name first to have the same order every time
	sort.Slice(secrets.Items, func(i, j int) bool {
		return strings.Compare(secrets.Items[i].Name, secrets.Items[j].Name) < 0
	})

	container := &specDeployment.Spec.Template.Spec.Containers[0]
	for _, secretObj := range secrets.Items {
		switch secretObj.Annotations[constants.CheEclipseOrgMountAs] {
		case "file":
			voluseSource := corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretObj.Name,
				},
			}

			volume := corev1.Volume{
				Name:         secretObj.Name,
				VolumeSource: voluseSource,
			}

			volumeMount := corev1.VolumeMount{
				Name:      secretObj.Name,
				MountPath: secretObj.Annotations[constants.CheEclipseOrgMountPath],
			}

			specDeployment.Spec.Template.Spec.Volumes = append(specDeployment.Spec.Template.Spec.Volumes, volume)
			container.VolumeMounts = append(container.VolumeMounts, volumeMount)

		case "env":
			secret := &corev1.Secret{}
			exists, err := GetNamespacedObject(deployContext, secretObj.Name, secret)
			if err != nil {
				return err
			} else if !exists {
				return fmt.Errorf("Secret '%s' not found", secretObj.Name)
			}

			// grab all keys and sort first to have the same order every time
			keys := make([]string, 0)
			for k := range secret.Data {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				return strings.Compare(keys[i], keys[j]) < 0
			})

			for _, key := range keys {
				var envName string

				// check if evn name defined explicitly
				envNameAnnotation := constants.CheEclipseOrg + "/" + key + "_env-name"
				envName, envNameExists := secretObj.Annotations[envNameAnnotation]
				if !envNameExists {
					// check if there is only one env name to mount
					envName, envNameExists = secretObj.Annotations[constants.CheEclipseOrgEnvName]
					if len(secret.Data) > 1 {
						return fmt.Errorf("There are more than one environment variable to mount. Use annotation '%s' to specify a name", envNameAnnotation)
					} else if !envNameExists {
						return fmt.Errorf("Environment name to mount secret key not found. Use annotation '%s' to specify a name", constants.CheEclipseOrgEnvName)
					}
				}

				env := corev1.EnvVar{
					Name: envName,
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: key,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretObj.Name,
							},
						},
					},
				}
				container.Env = append(container.Env, env)
			}
		}
	}

	return nil
}

// MountConfigMaps mounts configmaps into a container as a file or as environment variable.
// Configmaps are selected by the following labels:
// - app.kubernetes.io/part-of=che.eclipse.org
// - app.kubernetes.io/component=<DEPLOYMENT-NAME>-configmap
func MountConfigMaps(specDeployment *appsv1.Deployment, deployContext *chetypes.DeployContext) error {
	configmaps := &corev1.ConfigMapList{}

	kubernetesPartOfLabelSelectorRequirement, _ := labels.NewRequirement(constants.KubernetesPartOfLabelKey, selection.Equals, []string{constants.CheEclipseOrg})
	kubernetesComponentLabelSelectorRequirement, _ := labels.NewRequirement(constants.KubernetesComponentLabelKey, selection.Equals, []string{specDeployment.Name + "-configmap"})

	listOptions := &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*kubernetesPartOfLabelSelectorRequirement).Add(*kubernetesComponentLabelSelectorRequirement),
	}
	if err := deployContext.ClusterAPI.Client.List(context.TODO(), configmaps, listOptions); err != nil {
		return err
	}

	// sort configmaps by name first to have the same order every time
	sort.Slice(configmaps.Items, func(i, j int) bool {
		return strings.Compare(configmaps.Items[i].Name, configmaps.Items[j].Name) < 0
	})

	container := &specDeployment.Spec.Template.Spec.Containers[0]
	for _, configMapObj := range configmaps.Items {
		switch configMapObj.Annotations[constants.CheEclipseOrgMountAs] {
		case "file":
			voluseSource := corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapObj.Name,
					},
				},
			}

			volume := corev1.Volume{
				Name:         configMapObj.Name,
				VolumeSource: voluseSource,
			}

			volumeMount := corev1.VolumeMount{
				Name:      configMapObj.Name,
				MountPath: configMapObj.Annotations[constants.CheEclipseOrgMountPath],
			}

			specDeployment.Spec.Template.Spec.Volumes = append(specDeployment.Spec.Template.Spec.Volumes, volume)
			container.VolumeMounts = append(container.VolumeMounts, volumeMount)

		case "env":
			configmap := &corev1.ConfigMap{}
			exists, err := GetNamespacedObject(deployContext, configMapObj.Name, configmap)
			if err != nil {
				return err
			} else if !exists {
				return fmt.Errorf("ConfigMap '%s' not found", configMapObj.Name)
			}

			// grab all keys and sort first to have the same order every time
			keys := make([]string, 0)
			for k := range configmap.Data {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				return strings.Compare(keys[i], keys[j]) < 0
			})

			for _, key := range keys {
				var envName string

				// check if evn name defined explicitly
				envNameAnnotation := constants.CheEclipseOrg + "/" + key + "_env-name"
				envName, envNameExists := configMapObj.Annotations[envNameAnnotation]
				if !envNameExists {
					// check if there is only one env name to mount
					envName, envNameExists = configMapObj.Annotations[constants.CheEclipseOrgEnvName]
					if len(configmap.Data) > 1 {
						return fmt.Errorf("There are more than one environment variable to mount. Use annotation '%s' to specify a name", envNameAnnotation)
					} else if !envNameExists {
						return fmt.Errorf("Environment name to mount configmap key not found. Use annotation '%s' to specify a name", constants.CheEclipseOrgEnvName)
					}
				}

				env := corev1.EnvVar{
					Name: envName,
					ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: key,
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapObj.Name,
							},
						},
					},
				}
				container.Env = append(container.Env, env)
			}
		}
	}

	return nil
}
