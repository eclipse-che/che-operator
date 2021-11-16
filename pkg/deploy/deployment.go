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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eclipse-che/che-operator/pkg/util"
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
	deployContext *DeployContext,
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
		return false, err
	} else if !done {
		return util.IsTestMode(), nil
	}

	// always return true for tests
	if util.IsTestMode() {
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

// MountSecrets mounts secrets into a container as a file or as environment variable.
// Secrets are selected by the following labels:
// - app.kubernetes.io/part-of=che.eclipse.org
// - app.kubernetes.io/component=<DEPLOYMENT-NAME>-secret
func MountSecrets(specDeployment *appsv1.Deployment, deployContext *DeployContext) error {
	secrets := &corev1.SecretList{}

	kubernetesPartOfLabelSelectorRequirement, _ := labels.NewRequirement(KubernetesPartOfLabelKey, selection.Equals, []string{CheEclipseOrg})
	kubernetesComponentLabelSelectorRequirement, _ := labels.NewRequirement(KubernetesComponentLabelKey, selection.Equals, []string{specDeployment.Name + "-secret"})

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
		switch secretObj.Annotations[CheEclipseOrgMountAs] {
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
				MountPath: secretObj.Annotations[CheEclipseOrgMountPath],
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
				envNameAnnotation := CheEclipseOrg + "/" + key + "_env-name"
				envName, envNameExists := secretObj.Annotations[envNameAnnotation]
				if !envNameExists {
					// check if there is only one env name to mount
					envName, envNameExists = secretObj.Annotations[CheEclipseOrgEnvName]
					if len(secret.Data) > 1 {
						return fmt.Errorf("There are more than one environment variable to mount. Use annotation '%s' to specify a name", envNameAnnotation)
					} else if !envNameExists {
						return fmt.Errorf("Environment name to mount secret key not found. Use annotation '%s' to specify a name", CheEclipseOrgEnvName)
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
func MountConfigMaps(specDeployment *appsv1.Deployment, deployContext *DeployContext) error {
	configmaps := &corev1.ConfigMapList{}

	kubernetesPartOfLabelSelectorRequirement, _ := labels.NewRequirement(KubernetesPartOfLabelKey, selection.Equals, []string{CheEclipseOrg})
	kubernetesComponentLabelSelectorRequirement, _ := labels.NewRequirement(KubernetesComponentLabelKey, selection.Equals, []string{specDeployment.Name + "-configmap"})

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
		switch configMapObj.Annotations[CheEclipseOrgMountAs] {
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
				MountPath: configMapObj.Annotations[CheEclipseOrgMountPath],
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
				envNameAnnotation := CheEclipseOrg + "/" + key + "_env-name"
				envName, envNameExists := configMapObj.Annotations[envNameAnnotation]
				if !envNameExists {
					// check if there is only one env name to mount
					envName, envNameExists = configMapObj.Annotations[CheEclipseOrgEnvName]
					if len(configmap.Data) > 1 {
						return fmt.Errorf("There are more than one environment variable to mount. Use annotation '%s' to specify a name", envNameAnnotation)
					} else if !envNameExists {
						return fmt.Errorf("Environment name to mount configmap key not found. Use annotation '%s' to specify a name", CheEclipseOrgEnvName)
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
