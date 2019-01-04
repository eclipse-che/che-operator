//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package operator

import (
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func service(name string, labels map[string]string, portName string, portNumber int32) *corev1.Service {

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			Namespace: namespace,
			Labels:    labels,

		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     portName,
					Port:     portNumber,
					Protocol: "TCP",
				},
			},
			Selector: labels,
		},
	}
}
// CreateService creates a service with a given name, port, selector and labels
func CreateService(name string, labels map[string]string, portName string, portNumber int32) *corev1.Service {
	svc := service(name, labels, portName, portNumber)
	if err := sdk.Create(svc); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+name+" service : %v", err)
		return nil
	}
	return svc

}
