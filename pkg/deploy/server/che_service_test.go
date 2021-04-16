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
package server

import (
	"fmt"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
)

type DummyServiceCreator struct {
}

func (s *DummyServiceCreator) CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error {
	return nil
}

type DummyFailingServiceCreator struct {
}

func (s *DummyFailingServiceCreator) CreateService(cr *orgv1.CheCluster, service *corev1.Service, updateIfExists bool) error {
	return fmt.Errorf("dummy error")
}

func TestCreateCheDefaultService(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{},
		},
	}
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{},
	}
	service := GetSpecCheService(deployContext)
	ports := service.Spec.Ports
	if len(ports) != 1 {
		t.Error("expected 1 default port")
	}
	checkPort(ports[0], "http", 8080, t)
}

func TestCreateCheServerDebug(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheDebug: "true",
			},
		},
	}
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{},
	}

	service := GetSpecCheService(deployContext)
	ports := service.Spec.Ports
	if len(ports) != 2 {
		t.Error("expected 2 default port")
	}
	checkPort(ports[0], "http", 8080, t)
	checkPort(ports[1], "debug", 8000, t)
}

func TestCreateCheServiceEnableMetrics(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Metrics: orgv1.CheClusterSpecMetrics{
				Enable: false,
			},
		},
	}

	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{},
	}

	service := GetSpecCheService(deployContext)
	ports := service.Spec.Ports
	if len(ports) != 1 {
		t.Error("expected 1 default port")
	}
	checkPort(ports[0], "http", 8080, t)
}

func TestCreateCheServiceDisableMetrics(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		Spec: orgv1.CheClusterSpec{
			Metrics: orgv1.CheClusterSpecMetrics{
				Enable: true,
			},
		},
	}

	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{},
	}

	service := GetSpecCheService(deployContext)
	ports := service.Spec.Ports
	if len(ports) != 2 {
		t.Error("expected 2 ports")
	}
	checkPort(ports[0], "http", 8080, t)
	checkPort(ports[1], "metrics", deploy.DefaultCheMetricsPort, t)
}

func checkPort(actualPort corev1.ServicePort, expectedName string, expectedPort int32, t *testing.T) {
	if actualPort.Name != expectedName || actualPort.Port != expectedPort {
		t.Errorf("expected port name:`%s` port:`%d`, actual name:`%s` port:`%d`",
			expectedName, expectedPort, actualPort.Name, actualPort.Port)
	}
}
