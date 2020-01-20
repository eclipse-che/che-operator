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
)

type ServiceWriter interface {
	CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error
}

func NewCheService(instance *orgv1.CheCluster, cheLabels map[string]string, r ServiceWriter) (*corev1.Service, error) {
	portNames := []string{"http"}
	portPorts := []int32{8080}
	if instance.Spec.Metrics.Enable {
		portNames = append(portNames, "metrics")
		portPorts = append(portPorts, DefaultCheMetricsPort)
	}

	if instance.Spec.Server.CheDebug == "true" {
		portNames = append(portNames, "debug")
		portPorts = append(portPorts, DefaultCheDebugPort)
	}

	cheService := NewService(instance, "che-host", portNames, portPorts, cheLabels)
	if err := r.CreateService(instance, cheService, true); err != nil {
		return nil, err
	}
	return cheService, nil
}
