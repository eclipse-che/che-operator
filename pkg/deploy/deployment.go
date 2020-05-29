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
	"context"
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var deploymentDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(appsv1.Deployment{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(appsv1.DeploymentSpec{}, "Replicas", "RevisionHistoryLimit", "ProgressDeadlineSeconds"),
	cmpopts.IgnoreFields(appsv1.DeploymentStrategy{}, "RollingUpdate"),
	cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
	cmpopts.IgnoreFields(corev1.PodSpec{}, "DNSPolicy", "SchedulerName", "SecurityContext"),
	cmpopts.IgnoreFields(corev1.ConfigMapVolumeSource{}, "DefaultMode"),
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
	checluster *orgv1.CheCluster,
	specDeployment *appsv1.Deployment,
	clusterDeployment *appsv1.Deployment,
	additionalDeploymentDiffOpts cmp.Options,
	additionalDeploymentMerge func(*appsv1.Deployment, *appsv1.Deployment) *appsv1.Deployment,
	clusterAPI ClusterAPI) DeploymentProvisioningStatus {

	clusterDeployment, err := getClusterDeployment(specDeployment.Name, specDeployment.Namespace, clusterAPI.Client)
	if err != nil {
		return DeploymentProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterDeployment == nil {
		logrus.Infof("Creating a new object: %s, name %s", specDeployment.Kind, specDeployment.Name)
		err := clusterAPI.Client.Create(context.TODO(), specDeployment)
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
			logrus.Infof("Updating existed object: %s, name: %s", specDeployment.Kind, specDeployment.Name)
			fmt.Printf("Difference:\n%s", diff)
			clusterDeployment = additionalDeploymentMerge(specDeployment, clusterDeployment)
			err := clusterAPI.Client.Update(context.TODO(), clusterDeployment)
			return DeploymentProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
			}
		}
	}

	diff := cmp.Diff(clusterDeployment, specDeployment, deploymentDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", specDeployment.Kind, specDeployment.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterDeployment.Spec = specDeployment.Spec
		err := clusterAPI.Client.Update(context.TODO(), clusterDeployment)
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

func getClusterDeployment(name string, namespace string, client runtimeClient.Client) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return deployment, nil
}
