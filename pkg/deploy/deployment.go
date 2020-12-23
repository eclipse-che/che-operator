//
// Copyright (c) 2012-2020 Red Hat, Inc.
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
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"errors"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

var DeploymentDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(appsv1.Deployment{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas", "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
	cmpopts.IgnoreFields(appsv1.DeploymentStrategy{}, "RollingUpdate"),
	cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
	cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "SecurityContext", "DeprecatedServiceAccount"),
	cmpopts.IgnoreFields(corev1.ConfigMapVolumeSource{}, "DefaultMode"),
	cmpopts.IgnoreFields(corev1.SecretVolumeSource{}, "DefaultMode"),
	cmpopts.IgnoreFields(corev1.VolumeSource{}, "EmptyDir"),
	cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	}),
}

type DeploymentProvisioningStatus struct {
	ProvisioningStatus
	Deployment *appsv1.Deployment
}

func SyncDeploymentToCluster(
	deployContext *DeployContext,
	specDeployment *appsv1.Deployment,
	clusterDeployment *appsv1.Deployment,
	additionalDeploymentDiffOpts cmp.Options,
	additionalDeploymentMerge func(*appsv1.Deployment, *appsv1.Deployment) *appsv1.Deployment) DeploymentProvisioningStatus {

	if err := MountSecrets(specDeployment, deployContext); err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	clusterDeployment, err := GetClusterDeployment(specDeployment.Name, specDeployment.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterDeployment == nil {
		logrus.Infof("Creating a new object: %s, name %s", specDeployment.Kind, specDeployment.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specDeployment)
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	// 2-step comparation process
	// Firstly compare fields (and update the object if necessary) specifc to deployment
	// And only then compare common deployment fields
	if additionalDeploymentDiffOpts != nil {
		diff := cmp.Diff(clusterDeployment, specDeployment, additionalDeploymentDiffOpts)
		if len(diff) > 0 {
			logrus.Infof("Updating existing object: %s, name: %s", specDeployment.Kind, specDeployment.Name)
			fmt.Printf("Difference:\n%s", diff)
			clusterDeployment = additionalDeploymentMerge(specDeployment, clusterDeployment)
			err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterDeployment)
			return DeploymentProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
			}
		}
	}

	diff := cmp.Diff(clusterDeployment, specDeployment, DeploymentDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", specDeployment.Kind, specDeployment.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterDeployment.Spec = specDeployment.Spec
		err := deployContext.ClusterAPI.Client.Update(context.TODO(), clusterDeployment)
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	if clusterDeployment.Spec.Strategy.Type == appsv1.RollingUpdateDeploymentStrategyType && clusterDeployment.Status.Replicas > 1 {
		logrus.Infof("Deployment %s is in the rolling update state.", specDeployment.Name)
	}

	return DeploymentProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{Continue: clusterDeployment.Status.AvailableReplicas == 1 && clusterDeployment.Status.Replicas == 1},
		Deployment:         clusterDeployment,
	}
}

func GetClusterDeployment(name string, namespace string, client runtimeClient.Client) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}

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

	// sort secret first to have the same order every time
	sort.Slice(secrets.Items, func(i, j int) bool {
		return strings.Compare(secrets.Items[i].Name, secrets.Items[j].Name) < 0
	})

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
			specDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(specDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMount)

		case "env":
			secret, err := GetClusterSecret(secretObj.Name, deployContext.CheCluster.Namespace, deployContext.ClusterAPI)
			if err != nil {
				return err
			} else if secret == nil {
				return errors.New("Secret " + secretObj.Name + " not found.")
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
						return errors.New("There are more than one environment variable to mount. Use annotation '" + envNameAnnotation + "' to specify a name.")
					} else if !envNameExists {
						return errors.New("Environment name to mount secret key not found. Use annotation '" + CheEclipseOrgEnvName + "' to specify a name.")
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
				specDeployment.Spec.Template.Spec.Containers[0].Env = append(specDeployment.Spec.Template.Spec.Containers[0].Env, env)
			}
		}
	}

	return nil
}
