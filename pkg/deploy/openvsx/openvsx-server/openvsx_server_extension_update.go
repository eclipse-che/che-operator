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
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/openvsx"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *OpenVSXServerReconciler) syncExtensionUpdateCronJob(ctx *chetypes.DeployContext) error {
	if !ctx.CheCluster.IsExtensionAutoUpdateEnabled() {
		return deleteExtensionUpdateCronJob(ctx)
	}

	cronJob, err := r.getExtensionUpdateCronJobSpec(ctx)
	if err != nil {
		return fmt.Errorf("failed to get extension update CronJob spec: %w", err)
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, cronJob, ctx.ClusterAPI.Scheme); err != nil {
		return err
	}

	return ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		cronJob,
		&k8sclient.SyncOptions{DiffOpts: diffs.CronJob},
	)
}

func (r *OpenVSXServerReconciler) getExtensionUpdateCronJobSpec(ctx *chetypes.DeployContext) (*batchv1.CronJob, error) {
	autoUpdate := ctx.CheCluster.Spec.Components.OpenVSXRegistry.ExtensionAutoUpdate

	schedule := constants.DefaultExtensionAutoUpdateSchedule
	if autoUpdate.Schedule != nil && *autoUpdate.Schedule != "" {
		schedule = *autoUpdate.Schedule
	}

	image := defaults.GetOpenVSXImage(ctx.CheCluster)
	imagePullPolicy := utils.GetPullPolicyFromDockerImage(image)

	labels := deploy.GetLabels(constants.OpenVSXServerExtensionUpdateCronJobName)
	credentialsSecret := openvsx.GetCredentialsSecretName(ctx)

	env := []corev1.EnvVar{
		{
			Name:  "OVSX_REGISTRY_URL",
			Value: openvsx.GetOpenVSXServerServiceURL(ctx),
		},
		{
			Name:  "OVSX_FORWARDED_HOST",
			Value: ctx.CheHost,
		},
		{
			Name:  "OVSX_FORWARDED_PROTO",
			Value: "https",
		},
		utils.EnvVarFromSecret("OVSX_PAT", credentialsSecret, "openvsx-publisher-token"),
	}

	if autoUpdate.VSCodeEngineVersion != nil && *autoUpdate.VSCodeEngineVersion != "" {
		env = append(env, corev1.EnvVar{
			Name:  "VSCODE_ENGINE_VERSION",
			Value: *autoUpdate.VSCodeEngineVersion,
		})
	}

	if len(autoUpdate.ExcludeExtensions) > 0 {
		env = append(env, corev1.EnvVar{
			Name:  "EXCLUDE_EXTENSIONS",
			Value: strings.Join(autoUpdate.ExcludeExtensions, ","),
		})
	}

	cronJob := &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerExtensionUpdateCronJobName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   schedule,
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			SuccessfulJobsHistoryLimit: ptr.To(int32(1)),
			FailedJobsHistoryLimit:     ptr.To(int32(3)),
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            constants.OpenVSXServerExtensionUpdateCronJobName,
									Image:           image,
									ImagePullPolicy: corev1.PullPolicy(imagePullPolicy),
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "extensions",
											MountPath: "/home/openvsx/extensions",
											ReadOnly:  true,
										},
									},
									Env:     env,
									Command: []string{"/home/openvsx/update-extensions.sh", "/home/openvsx/extensions/extensions.list"},
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
					Parallelism:           ptr.To(int32(1)),
					BackoffLimit:          ptr.To(int32(3)),
					Completions:           ptr.To(int32(1)),
					ActiveDeadlineSeconds: ptr.To(int64(1800)),
				},
			},
		},
	}

	deploy.EnsurePodSecurityStandards(
		&cronJob.Spec.JobTemplate.Spec.Template.Spec,
		constants.DefaultSecurityContextRunAsUser,
		constants.DefaultSecurityContextFsGroup,
	)

	return cronJob, nil
}

func deleteExtensionUpdateCronJob(ctx *chetypes.DeployContext) error {
	return ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXServerExtensionUpdateCronJobName,
			Namespace: ctx.CheCluster.Namespace,
		},
		&batchv1.CronJob{},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)
}
