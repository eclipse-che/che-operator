//
// Copyright (c) 2020 Red Hat, Inc.
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
	"reflect"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	jobDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(batchv1.Job{}, "TypeMeta", "ObjectMeta", "Status"),
		cmpopts.IgnoreFields(batchv1.JobSpec{}, "Selector", "TTLSecondsAfterFinished"),
		cmpopts.IgnoreFields(v1.PodTemplateSpec{}, "ObjectMeta"),
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
	deployContext *DeployContext,
	name string,
	component string,
	image string,
	serviceAccountName string,
	env map[string]string) (*batchv1.Job, error) {

	specJob, err := getSpecJob(deployContext, name, component, image, serviceAccountName, env)
	if err != nil {
		return nil, err
	}

	clusterJob, err := getClusterJob(specJob.Name, specJob.Namespace, deployContext.ClusterAPI)
	if err != nil {
		return nil, err
	}

	if clusterJob == nil {
		logrus.Infof("Creating a new object: %s, name %s", specJob.Kind, specJob.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specJob)
		return nil, err
	}

	diff := cmp.Diff(clusterJob, specJob, jobDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterJob.Kind, clusterJob.Name)
		fmt.Printf("Difference:\n%s", diff)

		if err := deployContext.ClusterAPI.Client.Delete(context.TODO(), clusterJob); err != nil {
			return nil, err
		}

		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specJob)
		return nil, err
	}

	return clusterJob, nil
}

// GetSpecJob creates new job configuration by given parameters.
func getSpecJob(
	deployContext *DeployContext,
	name string,
	component string,
	image string,
	serviceAccountName string,
	env map[string]string) (*batchv1.Job, error) {
	labels := GetLabels(deployContext.CheCluster, component)
	backoffLimit := int32(3)
	parallelism := int32(1)
	comletions := int32(1)
	terminationGracePeriodSeconds := int64(30)
	ttlSecondsAfterFinished := int32(30)
	pullPolicy := corev1.PullPolicy(util.GetValue(string(deployContext.CheCluster.Spec.Server.CheImagePullPolicy), "IfNotPresent"))

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
					DeprecatedServiceAccount:      serviceAccountName,
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

	if err := controllerutil.SetControllerReference(deployContext.CheCluster, job, deployContext.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

// GetClusterJob gets and returns specified job
func getClusterJob(name string, namespace string, clusterAPI ClusterAPI) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}
