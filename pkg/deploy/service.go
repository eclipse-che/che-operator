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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
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
	CheServiceHame = "che-host"
)

var portsDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.ServicePort{}, "TargetPort", "NodePort"),
}

func SyncCheServiceToCluster(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) ServiceProvisioningStatus {
	specService, err := GetSpecCheService(checluster, clusterAPI)
	if err != nil {
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	return doSyncServiceToCluster(checluster, specService, clusterAPI)
}

func GetSpecCheService(checluster *orgv1.CheCluster, clusterAPI ClusterAPI) (*corev1.Service, error) {
	portName := []string{"http"}
	portNumber := []int32{8080}
	labels := GetLabels(checluster, DefaultCheFlavor(checluster))

	if checluster.Spec.Metrics.Enable {
		portName = append(portName, "metrics")
		portNumber = append(portNumber, DefaultCheMetricsPort)
	}

	if checluster.Spec.Server.CheDebug == "true" {
		portName = append(portName, "debug")
		portNumber = append(portNumber, DefaultCheDebugPort)
	}

	return getSpecService(checluster, CheServiceHame, portName, portNumber, labels, clusterAPI)
}

func SyncServiceToCluster(
	checluster *orgv1.CheCluster,
	name string,
	portName []string,
	portNumber []int32,
	labels map[string]string,
	clusterAPI ClusterAPI) ServiceProvisioningStatus {
	specService, err := getSpecService(checluster, name, portName, portNumber, labels, clusterAPI)
	if err != nil {
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	return doSyncServiceToCluster(checluster, specService, clusterAPI)
}

func doSyncServiceToCluster(
	checluster *orgv1.CheCluster,
	specService *corev1.Service,
	clusterAPI ClusterAPI) ServiceProvisioningStatus {

	clusterService, err := getClusterService(specService.Name, specService.Namespace, clusterAPI.Client)
	if err != nil {
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterService == nil {
		logrus.Infof("Creating a new object: %s, name %s", specService.Kind, specService.Name)
		err := clusterAPI.Client.Create(context.TODO(), specService)
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

		err := clusterAPI.Client.Delete(context.TODO(), clusterService)
		if err != nil {
			return ServiceProvisioningStatus{
				ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
			}
		}

		err = clusterAPI.Client.Create(context.TODO(), specService)
		return ServiceProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	return ServiceProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{Continue: true},
	}
}

func getSpecService(
	checluster *orgv1.CheCluster,
	name string,
	portName []string,
	portNumber []int32,
	labels map[string]string,
	clusterAPI ClusterAPI) (*corev1.Service, error) {

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
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}

	if !util.IsTestMode() {
		err := controllerutil.SetControllerReference(checluster, service, clusterAPI.Scheme)
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
