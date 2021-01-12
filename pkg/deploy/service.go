//
// Copyright (c) 2020-2020 Red Hat, Inc.
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

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ServiceProvisioningStatus struct {
	ProvisioningStatus
}

const (
	CheServiceName = "che-host"
)

var portsDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ServicePort{}, "TargetPort", "NodePort"),
}

func SyncServiceToCluster(
	deployContext *DeployContext,
	name string,
	portName []string,
	portNumber []int32,
	component string) ServiceProvisioningStatus {
	specService, err := GetSpecService(deployContext, name, portName, portNumber, component)
	if err != nil {
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	return DoSyncServiceToCluster(deployContext, specService)
}

func DoSyncServiceToCluster(deployContext *DeployContext, specService *corev1.Service) ServiceProvisioningStatus {

	clusterService, err := getClusterService(specService.Name, specService.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterService == nil {
		logrus.Infof("Creating a new object: %s, name %s", specService.Kind, specService.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specService)
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	diffPorts := cmp.Diff(clusterService.Spec.Ports, specService.Spec.Ports, portsDiffOpts)
	diffSelectors := cmp.Diff(clusterService.Spec.Selector, specService.Spec.Selector)
	if len(diffPorts) > 0 || len(diffSelectors) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", specService.Kind, specService.Name)
		fmt.Printf("Ports difference:\n%s", diffPorts)
		fmt.Printf("Selectors difference:\n%s", diffSelectors)

		err := deployContext.ClusterAPI.Client.Delete(context.TODO(), clusterService)
		if err != nil {
			return ServiceProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
			}
		}

		err = deployContext.ClusterAPI.Client.Create(context.TODO(), specService)
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	return ServiceProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{Continue: true},
	}
}

func GetSpecService(
	deployContext *DeployContext,
	name string,
	portName []string,
	portNumber []int32,
	component string) (*corev1.Service, error) {

	labels, selector := GetLabelsAndSelector(deployContext.CheCluster, component)
	ports := []corev1.ServicePort{}
	for i := range portName {
		port := corev1.ServicePort{
			Name:     portName[i],
			Port:     portNumber[i],
			Protocol: "TCP",
		}
		ports = append(ports, port)
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: selector,
		},
	}

	if !util.IsTestMode() {
		err := controllerutil.SetControllerReference(deployContext.CheCluster, service, deployContext.ClusterAPI.Scheme)
		if err != nil {
			return nil, err
		}
	}

	return service, nil
}

func getClusterService(name string, namespace string, client runtimeClient.Client) (*corev1.Service, error) {
	service := &corev1.Service{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, service)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return service, nil
}
