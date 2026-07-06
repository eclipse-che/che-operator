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

package openvsx_server

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *OpenVSXServerReconciler) syncDefaultExtensionsConfig(ctx *chetypes.DeployContext) error {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerExtensionsConfigMapName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerComponentName),
		},
		Data: map[string]string{
			"extensions.list": "",
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, cm, ctx.ClusterAPI.Scheme); err != nil {
		return err
	}

	return ctx.ClusterAPI.ClientWrapper.CreateIfNotExists(context.TODO(), cm)
}

func (r *OpenVSXServerReconciler) getExtensionsVersion(ctx *chetypes.DeployContext) (string, error) {
	cm := &corev1.ConfigMap{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXServerExtensionsConfigMapName,
			Namespace: ctx.CheCluster.Namespace,
		},
		cm,
	)
	if err != nil {
		return "", fmt.Errorf("failed to get ConfigMap: %w", err)
	}
	if !exists {
		return "", nil
	}

	return cm.ResourceVersion, nil
}

func (r *OpenVSXServerReconciler) syncExtensions(ctx *chetypes.DeployContext) (bool, error) {
	image := defaults.GetOpenVSXImage(ctx.CheCluster)
	imagePullPolicy := utils.GetPullPolicyFromDockerImage(image)

	labels := deploy.GetLabels(constants.OpenVSXServerComponentName)

	credentialsSecret := openvsx.GetCredentialsSecretName(ctx)

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerExtensionPublishJobName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            constants.OpenVSXServerExtensionPublishJobName,
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "extensions",
									MountPath: "/home/openvsx/extensions",
									ReadOnly:  true,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.OpenVSXServerPublisherMemoryRequest),
									corev1.ResourceCPU:    resource.MustParse(constants.OpenVSXServerPublisherCpuRequest),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse(constants.OpenVSXServerPublisherMemoryLimit),
									corev1.ResourceCPU:    resource.MustParse(constants.OpenVSXServerPublisherCpuLimit),
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "OVSX_REGISTRY_URL",
									Value: openvsx.GetOpenVSXServerServiceURL(ctx),
								},
								utils.EnvVarFromSecret("OVSX_PAT", credentialsSecret, "openvsx-publisher-token"),
							},
							Command: []string{"/home/openvsx/publish-extensions.sh", "/home/openvsx/extensions/extensions.list"},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "extensions",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: constants.OpenVSXServerExtensionsConfigMapName,
									},
								},
							},
						},
					},
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: ptr.To(int64(30)),
				},
			},
			Parallelism:             ptr.To(int32(1)),
			BackoffLimit:            ptr.To(int32(3)),
			Completions:             ptr.To(int32(1)),
			TTLSecondsAfterFinished: ptr.To(int32(300)),
			ActiveDeadlineSeconds:   ptr.To(int64(300)),
		},
	}

	deploy.EnsurePodSecurityStandards(
		&job.Spec.Template.Spec,
		constants.DefaultSecurityContextRunAsUser,
		constants.DefaultSecurityContextFsGroup,
	)

	if err := controllerutil.SetControllerReference(ctx.CheCluster, job, ctx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		job,
		&k8sclient.SyncOptions{
			DeleteOpts: []client.DeleteOption{client.PropagationPolicy(metav1.DeletePropagationBackground)},
		},
	)
	if errors.IsAlreadyExists(err) {
		return false, nil
	}

	return err == nil, err
}
