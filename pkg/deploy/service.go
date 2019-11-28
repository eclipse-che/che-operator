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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewService(cr *orgv1.CheCluster, name string, portName []string, portNumber []int32, labels map[string]string) *corev1.Service {
	ports := []corev1.ServicePort{}
	for i := range portName {
		port := corev1.ServicePort{
			Name:     portName[i],
			Port:     portNumber[i],
			Protocol: "TCP",
		}
		ports = append(ports, port)
	}
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports:    ports,
			Selector: labels,
		},
	}
}

// Writes only mutable fields from `in` into `out` service.
//
// This is useful when doing service update, because we have to get existing service,
// update just mutable fields and leave the rest as is.
//
// Be aware that function is not doing any copy/deepcopy.
func MergeServices(out *corev1.Service, in *corev1.Service) {
	out.ObjectMeta.Labels = in.ObjectMeta.Labels
	out.Spec.Ports = in.Spec.Ports
	out.Spec.Selector = in.Spec.Selector
}
