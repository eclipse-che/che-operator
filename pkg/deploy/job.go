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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var jobDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(batchv1.Job{}, "TypeMeta", "ObjectMeta", "Status"),
}

// SyncJobToCluster deploys new instance of given job
func SyncJobToCluster(instance *orgv1.CheCluster, job *batchv1.Job, clusterAPI ClusterAPI) error {
	jobFound, err := GetClusterJob(job.ObjectMeta.Name, job.ObjectMeta.Namespace, clusterAPI)
	if jobFound == nil && err == nil {
		logrus.Infof("Creating a new object: %s, name: %s", job.Kind, job.Name)
		err = clusterAPI.Client.Create(context.TODO(), job)
		if err != nil {
			logrus.Errorf("Failed to create %s %s: %s", job.Name, job.Kind, err)
			return err
		}
		return nil
	} else if err != nil {
		logrus.Errorf("An error occurred: %s", err)
		return err
	}

	diff := cmp.Diff(job, jobFound, jobDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", job.Kind, job.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := clusterAPI.Client.Delete(context.TODO(), jobFound)
		if err != nil {
			logrus.Errorf("Failed to update %s %s: %s", jobFound.Name, job.Kind, err)
			return err
		}

		err = clusterAPI.Client.Create(context.TODO(), job)
		if err != nil {
			logrus.Errorf("Failed to update %s %s: %s", job.Name, job.Kind, err)
			return err
		}
	}

	return nil
}

// GetSpecJob creates new job configuration by giben parameters.
func GetSpecJob(checluster *orgv1.CheCluster, name string, namespace string, image string, serviceAccountName string, env map[string]string, backoffLimit int32, clusterAPI ClusterAPI) (*batchv1.Job, error) {
	labels := GetLabels(checluster, util.GetValue(checluster.Spec.Server.CheFlavor, DefaultCheFlavor))
	labels["component"] = "che-create-tls-secret-job"

	pullPolicy := corev1.PullPolicy(util.GetValue(string(checluster.Spec.Server.CheImagePullPolicy), "IfNotPresent"))

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
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					RestartPolicy:      "Never",
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
			BackoffLimit: &backoffLimit,
		},
	}

	if err := controllerutil.SetControllerReference(checluster, job, clusterAPI.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

// GetClusterJob gets and returns specified job
func GetClusterJob(name string, namespace string, clusterAPI ClusterAPI) (*batchv1.Job, error) {
	job := &batchv1.Job{}
	err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return job, nil
}
