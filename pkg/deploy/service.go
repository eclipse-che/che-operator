//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CheServiceName = "che-host"
)

var ServiceDefaultDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Service{}, "TypeMeta", "ObjectMeta"),
	cmp.Comparer(func(x, y corev1.ServiceSpec) bool {
		return cmp.Equal(x.Ports, y.Ports, cmpopts.IgnoreFields(corev1.ServicePort{}, "TargetPort", "NodePort")) &&
			reflect.DeepEqual(x.Selector, y.Selector)
	}),
}

func SyncServiceToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	portName []string,
	portNumber []int32,
	component string) (bool, error) {

	serviceSpec := GetServiceSpec(deployContext, name, portName, portNumber, component)
	return SyncServiceSpecToCluster(deployContext, serviceSpec)
}

func SyncServiceSpecToCluster(deployContext *chetypes.DeployContext, serviceSpec *corev1.Service) (bool, error) {
	return Sync(deployContext, serviceSpec, ServiceDefaultDiffOpts)
}

func GetServiceSpec(
	deployContext *chetypes.DeployContext,
	name string,
	portName []string,
	portNumber []int32,
	component string) *corev1.Service {

	labels := GetLabels(component)
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
			Selector: labels,
		},
	}

	return service
}
