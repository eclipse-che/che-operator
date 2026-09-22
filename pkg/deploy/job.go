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

package deploy

import (
	"fmt"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	JobDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(batchv1.Job{}, "TypeMeta", "ObjectMeta", "Status"),
		cmpopts.IgnoreFields(batchv1.JobSpec{}, "Selector", "TTLSecondsAfterFinished"),
		cmpopts.IgnoreFields(corev1.PodTemplateSpec{}, "ObjectMeta"),
		cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
		cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "SecurityContext"),
		cmp.Comparer(func(x, y []corev1.EnvVar) bool {
			xMap := make(map[string]string)
			yMap := make(map[string]string)
			for _, env := range x {
				xMap[env.Name] = env.Value
			}
			for _, env := range y {
				yMap[env.Name] = env.Value
			}
			return reflect.DeepEqual(xMap, yMap)
		}),
	}
)

func SyncJobToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	component string,
	image string,
	serviceAccountName string,
	env map[string]string) error {

	jobSpec := getJobSpec(deployContext, name, component, image, serviceAccountName, env)

	if err := controllerutil.SetControllerReference(deployContext.CheCluster, jobSpec, deployContext.ClusterAPI.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for Job %s/%s: %w", jobSpec.Namespace, jobSpec.Name, err)
	}

	if err := deployContext.ClusterAPI.ClientWrapper.Sync(
		deployContext.Context,
		jobSpec,
		&k8sclient.SyncOptions{DiffOpts: JobDiffOpts},
	); err != nil {
		return fmt.Errorf("failed to sync Job %s/%s: %w", jobSpec.Namespace, jobSpec.Name, err)
	}

	return nil
}

// GetSpecJob creates new job configuration by given parameters.
func getJobSpec(
	deployContext *chetypes.DeployContext,
	name string,
	component string,
	image string,
	serviceAccountName string,
	env map[string]string) *batchv1.Job {
	labels := GetLabels(component)
	backoffLimit := int32(3)
	parallelism := int32(1)
	comletions := int32(1)
	terminationGracePeriodSeconds := int64(30)
	ttlSecondsAfterFinished := int32(30)
	pullPolicy := corev1.PullPolicy(utils.GetPullPolicyFromDockerImage(image))

	var jobEnvVars []corev1.EnvVar
	for envVarName, envVarValue := range env {
		jobEnvVars = append(jobEnvVars, corev1.EnvVar{Name: envVarName, Value: envVarValue})
	}

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            serviceAccountName,
					RestartPolicy:                 "Never",
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []corev1.Container{
						{
							Name:            name + "-job-container",
							Image:           image,
							ImagePullPolicy: pullPolicy,
							Env:             jobEnvVars,
						},
					},
				},
			},
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Parallelism:             &parallelism,
			BackoffLimit:            &backoffLimit,
			Completions:             &comletions,
		},
	}

	return job
}
