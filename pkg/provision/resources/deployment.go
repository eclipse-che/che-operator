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

package resources

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"time"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/cheerrors"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var Deployment = &deploymentHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(appsv1.Deployment{}, "TypeMeta", "Status"),
		cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas", "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
		cmpopts.IgnoreFields(appsv1.DeploymentStrategy{}, "RollingUpdate"),
		cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
		cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "DeprecatedServiceAccount"),
		cmpopts.IgnoreFields(corev1.SecretVolumeSource{}, "DefaultMode"),
		cmpopts.IgnoreFields(corev1.ConfigMapVolumeSource{}, "DefaultMode"),
		cmp.Comparer(func(x, y resource.Quantity) bool {
			return x.Cmp(y) == 0
		}),
	},
}

type deploymentHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (d *deploymentHandler) Sync(cheCtx *chetypes.CheContext, deployment *appsv1.Deployment) error {
	if err := d.mountSecrets(cheCtx, deployment); err != nil {
		return fmt.Errorf("failed to mount Secret into Deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
	}

	if err := d.mountConfigMaps(cheCtx, deployment); err != nil {
		return fmt.Errorf("failed to mount ConfigMap into Deployment %s/%s: %w", deployment.Namespace, deployment.Name, err)
	}

	actual := &appsv1.Deployment{}
	exists, err := d.Get(cheCtx, deployment.Name, deployment.Namespace, actual)
	if err != nil {
		return err
	} else if exists {
		// keep the replicas count
		deployment.Spec.Replicas = actual.Spec.Replicas
	}

	err = syncher.SyncObject(
		cheCtx,
		deployment,
		d.diff,
		d.suppressDiff,
		d.allowForceRecreate,
	)
	if err != nil {
		return err
	}

	exists, err = d.Get(cheCtx, deployment.Name, deployment.Namespace, actual)
	if err != nil {
		return err
	} else if !exists {
		return cheerrors.NewNotReadyError(deployment, time.Second)
	}

	isReady := actual.Status.ObservedGeneration >= actual.Generation && actual.Status.UnavailableReplicas == 0
	if !isReady {
		return cheerrors.NewNotReadyError(deployment, 10*time.Second)
	}

	return nil
}

func (d *deploymentHandler) Get(cheCtx *chetypes.CheContext, name string, namespace string, deployment *appsv1.Deployment) (bool, error) {
	return syncher.GetObject(
		cheCtx,
		name,
		namespace,
		deployment,
	)
}

func (d *deploymentHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return syncher.Delete(cheCtx, name, namespace, &appsv1.Deployment{})
}

func (d *deploymentHandler) ApplyOverrides(deployment *appsv1.Deployment, overrides *chev2.Deployment) {
	if overrides == nil {
		return
	}

	if len(overrides.Containers) > 0 {
		for index := range deployment.Spec.Template.Spec.Containers {
			container := &deployment.Spec.Template.Spec.Containers[index]

			if len(deployment.Spec.Template.Spec.Containers) == 1 {
				Container.ApplyOverrides(container, &overrides.Containers[0])
			} else {
				overrideContainer := utils.Find(
					overrides.Containers,
					func(overrideContainer chev2.Container) bool {
						return overrideContainer.Name == container.Name
					})

				Container.ApplyOverrides(container, overrideContainer)
			}
		}
	}

	if len(overrides.NodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = maps.Clone(overrides.NodeSelector)
	}

	if len(overrides.Tolerations) > 0 {
		deployment.Spec.Template.Spec.Tolerations = slices.Clone(overrides.Tolerations)
	}

	for i := range overrides.Volumes {
		overrideVolume := &overrides.Volumes[i]

		existing := utils.Find(
			deployment.Spec.Template.Spec.Volumes,
			func(vol corev1.Volume) bool {
				return vol.Name == overrideVolume.Name
			},
		)
		if existing == nil {
			deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, *overrideVolume.DeepCopy())
		} else {
			overrideVolume.DeepCopyInto(existing)
		}
	}

	if overrides.SecurityContext != nil {
		if !infrastructure.IsOpenShift() {
			if deployment.Spec.Template.Spec.SecurityContext == nil {
				deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
			}
			if overrides.SecurityContext.FsGroup != nil {
				deployment.Spec.Template.Spec.SecurityContext.FSGroup = new(*overrides.SecurityContext.FsGroup)
			}
			if overrides.SecurityContext.RunAsUser != nil {
				deployment.Spec.Template.Spec.SecurityContext.RunAsUser = new(*overrides.SecurityContext.RunAsUser)
			}
		}
	}
}

// trimCPULimitIfNoLimitRange removes the CPU limit of every container when the namespace
// has no LimitRange. It must be called before ApplyOverrides, so that a CPU limit overridden
// by the user is set afterwards and survives the trim.
// See details at https://github.com/eclipse/che/issues/22198
func (d *deploymentHandler) TrimCPULimitIfNoLimitRange(cheCtx *chetypes.CheContext, deployment *appsv1.Deployment) error {
	limitRanges, err := LimitRange.List(cheCtx, deployment.Namespace)
	if err != nil {
		return err
	} else if len(limitRanges) == 0 {
		for index := range deployment.Spec.Template.Spec.Containers {
			container := &deployment.Spec.Template.Spec.Containers[index]
			delete(container.Resources.Limits, corev1.ResourceCPU)
		}
	}

	return nil
}

// mountSecrets mounts Secrets labeled for this deployment:
// - app.kubernetes.io/part-of=che.eclipse.org
// - app.kubernetes.io/component=<DEPLOYMENT-NAME>-secret
func (d *deploymentHandler) mountSecrets(cheCtx *chetypes.CheContext, deployment *appsv1.Deployment) error {
	secrets, err := Secret.List(
		cheCtx,
		cheCtx.CheCluster.Namespace,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: deployment.Name + "-secret",
		},
		nil)
	if err != nil {
		return err
	}

	slices.SortFunc(
		secrets,
		func(a, b corev1.Secret) int {
			return strings.Compare(a.Name, b.Name)
		})

	pod := &deployment.Spec.Template.Spec
	for _, secret := range secrets {
		d.mountResource(pod, fromSecret(&secret))
	}

	return nil
}

// mountConfigMaps mounts ConfigMaps labeled for this deployment
// - app.kubernetes.io/part-of=che.eclipse.org
// - app.kubernetes.io/component=<DEPLOYMENT-NAME>-configmap
func (d *deploymentHandler) mountConfigMaps(cheCtx *chetypes.CheContext, deployment *appsv1.Deployment) error {
	configMaps, err := ConfigMap.List(
		cheCtx,
		cheCtx.CheCluster.Namespace,
		map[string]string{
			constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
			constants.KubernetesComponentLabelKey: deployment.Name + "-configmap",
		},
		nil)
	if err != nil {
		return err
	}

	slices.SortFunc(
		configMaps,
		func(a, b corev1.ConfigMap) int {
			return strings.Compare(a.Name, b.Name)
		})

	pod := &deployment.Spec.Template.Spec
	for _, configMap := range configMaps {
		d.mountResource(pod, fromConfigMap(&configMap))
	}

	return nil
}

func (d *deploymentHandler) mountResource(podSpec *corev1.PodSpec, resource mountable) {
	switch resource.annotations[constants.CheEclipseOrgMountAs] {
	case "file":
		d.mountAsFile(podSpec, resource)
	case "subpath":
		d.mountAsSubPath(podSpec, resource)
	case "env":
		d.mountAsEnv(podSpec, resource)
	}
}

func (d *deploymentHandler) mountAsFile(podSpec *corev1.PodSpec, resource mountable) {
	podSpec.Volumes = append(
		podSpec.Volumes,
		corev1.Volume{
			Name:         resource.name,
			VolumeSource: resource.volumeSource(),
		},
	)

	podSpec.Containers[0].VolumeMounts = append(
		podSpec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			Name:      resource.name,
			MountPath: resource.annotations[constants.CheEclipseOrgMountPath],
		},
	)
}

func (d *deploymentHandler) mountAsSubPath(podSpec *corev1.PodSpec, resource mountable) {
	podSpec.Volumes = append(
		podSpec.Volumes,
		corev1.Volume{
			Name:         resource.name,
			VolumeSource: resource.volumeSource(),
		})

	for _, key := range resource.dataKeys {
		mountPath := path.Join("/", resource.annotations[constants.CheEclipseOrgMountPath], key)

		podSpec.Containers[0].VolumeMounts = append(
			podSpec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      resource.name,
				MountPath: mountPath,
				SubPath:   key,
			},
		)
	}
}

func (d *deploymentHandler) mountAsEnv(podSpec *corev1.PodSpec, resource mountable) {
	for _, key := range resource.dataKeys {
		podSpec.Containers[0].Env = append(
			podSpec.Containers[0].Env,
			corev1.EnvVar{
				Name:      resource.envName(key),
				ValueFrom: resource.envVarSource(key),
			},
		)
	}
}

// mountable adapts a Secret or a ConfigMap for mounting into a pod,
// so that the mounting logic can be shared between both kinds.
type mountable struct {
	kind        string
	name        string
	namespace   string
	dataKeys    []string
	annotations map[string]string
}

func fromSecret(secret *corev1.Secret) mountable {
	return mountable{
		kind:        "Secret",
		name:        secret.Name,
		namespace:   secret.Namespace,
		dataKeys:    slices.Sorted(maps.Keys(secret.Data)),
		annotations: secret.Annotations,
	}
}

func fromConfigMap(configMap *corev1.ConfigMap) mountable {
	return mountable{
		kind:        "ConfigMap",
		name:        configMap.Name,
		namespace:   configMap.Namespace,
		dataKeys:    slices.Sorted(maps.Keys(configMap.Data)),
		annotations: configMap.Annotations,
	}
}

// envName resolves the environment variable name for the given data key:
//  1. the key-specific 'che.eclipse.org/<key>_env-name' annotation, if set;
//  2. the fallback 'che.eclipse.org/env-name' annotation, but only when the
//     resource has a single data key;
//  3. otherwise, the data key itself.
func (m mountable) envName(key string) string {
	keyEnvNameAnnotation := constants.CheEclipseOrg + "/" + key + "_env-name"
	if envName, exists := m.annotations[keyEnvNameAnnotation]; exists {
		return envName
	}

	if len(m.dataKeys) > 1 {
		return key
	}

	envName, exists := m.annotations[constants.CheEclipseOrgEnvName]
	if exists {
		return envName
	}

	return key
}

func (m mountable) volumeSource() corev1.VolumeSource {
	if m.kind == "ConfigMap" {
		return VolumeSource.FromConfigMap(m.name)
	}
	return VolumeSource.FromSecret(m.name)
}

func (m mountable) envVarSource(key string) *corev1.EnvVarSource {
	if m.kind == "ConfigMap" {
		return &corev1.EnvVarSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				Key: key,
				LocalObjectReference: corev1.LocalObjectReference{
					Name: m.name,
				},
			},
		}
	}

	return &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			Key: key,
			LocalObjectReference: corev1.LocalObjectReference{
				Name: m.name,
			},
		},
	}

}
