//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"context"
	"os"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestSyncService(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      os.Getenv("CHE_FLAVOR"),
			},
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					CheDebug: "true",
				},
				Metrics: orgv1.CheClusterSpecMetrics{
					Enable: true,
				},
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	server := NewServer(deployContext)
	done, err := server.SyncCheService()
	if !done {
		if err != nil {
			t.Fatalf("Failed to sync service, error: %v", err)
		} else {
			t.Fatalf("Failed to sync service")
		}
	}

	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.CheServiceName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service, error: %v", err)
	}

	checkPort(service.Spec.Ports[0], "http", 8080, t)
	checkPort(service.Spec.Ports[1], "metrics", deploy.DefaultCheMetricsPort, t)
	checkPort(service.Spec.Ports[2], "debug", deploy.DefaultCheDebugPort, t)
}

func TestSyncAll(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      os.Getenv("CHE_FLAVOR"),
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				TlsSupport: true,
			},
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	routev1.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
		Proxy: &deploy.Proxy{},
	}

	util.IsOpenShift = true

	server := NewServer(deployContext)
	done, err := server.ExposeCheServiceAndEndpoint()
	if !done || err != nil {
		t.Fatalf("Failed to sync Server: %v", err)
	}

	done, err = server.SyncAll()
	if !done || err != nil {
		t.Fatalf("Failed to sync Server: %v", err)
	}

	// check service
	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.CheServiceName, Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Service not found: %v", err)
	}

	// check endpoint
	route := &routev1.Route{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: server.component, Namespace: "eclipse-che"}, route)
	if err != nil {
		t.Fatalf("Route not found: %v", err)
	}

	// check configmap
	configMap := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: CheConfigMapName, Namespace: "eclipse-che"}, configMap)
	if err != nil {
		t.Fatalf("ConfigMap not found: %v", err)
	}

	// check deployment
	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: server.component, Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}

	if cheCluster.Status.CheURL == "" {
		t.Fatalf("CheURL is not set")
	}

	if cheCluster.Status.CheClusterRunning == "" {
		t.Fatalf("CheClusterRunning is not set")
	}

	if cheCluster.Status.CheVersion == "" {
		t.Fatalf("CheVersion is not set")
	}
}

func TestSyncLegacyConfigMap(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				TlsSupport: true,
			},
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	routev1.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
		Proxy: &deploy.Proxy{},
	}

	legacyConfigMap := deploy.GetConfigMapSpec(deployContext, "custom", map[string]string{"a": "b"}, "test")
	err := cli.Create(context.TODO(), legacyConfigMap)
	if err != nil {
		t.Fatalf("Failed to create config map: %v", err)
	}

	server := NewServer(deployContext)
	done, err := server.SyncLegacyConfigMap()
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	err = cli.Get(context.TODO(), types.NamespacedName{Namespace: "eclipse-che", Name: "custom"}, &corev1.ConfigMap{})
	if err == nil {
		t.Fatalf("Legacy configmap must be removed")
	}

	if cheCluster.Spec.Server.CustomCheProperties["a"] != "b" {
		t.Fatalf("CheCluster wasn't updated with legacy configmap data")
	}
}

func TestUpdateAvailabilityStatus(t *testing.T) {
	cheDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: "eclipse-che",
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
			Replicas:          1,
		},
	}
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      os.Getenv("CHE_FLAVOR"),
		},
		Spec:   orgv1.CheClusterSpec{},
		Status: orgv1.CheClusterStatus{},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	routev1.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cheCluster)
	deployContext := &deploy.DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	server := NewServer(deployContext)
	_, err := server.UpdateAvailabilityStatus()
	if err != nil {
		t.Fatalf("Failed to update availability status: %v", err)
	}
	if cheCluster.Status.CheClusterRunning != UnavailableStatus {
		t.Fatalf("Expected status: %s, actual: %s", UnavailableStatus, cheCluster.Status.CheClusterRunning)
	}

	err = cli.Create(context.TODO(), cheDeployment)
	if err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}
	_, err = server.UpdateAvailabilityStatus()
	if err != nil {
		t.Fatalf("Failed to update availability status: %v", err)
	}

	if cheCluster.Status.CheClusterRunning != AvailableStatus {
		t.Fatalf("Expected status: %s, actual: %s", AvailableStatus, cheCluster.Status.CheClusterRunning)
	}

	cheDeployment.Status.Replicas = 2
	err = cli.Update(context.TODO(), cheDeployment)
	if err != nil {
		t.Fatalf("Failed to update deployment: %v", err)
	}

	_, err = server.UpdateAvailabilityStatus()
	if err != nil {
		t.Fatalf("Failed to update availability status: %v", err)
	}
	if cheCluster.Status.CheClusterRunning != RollingUpdateInProgressStatus {
		t.Fatalf("Expected status: %s, actual: %s", RollingUpdateInProgressStatus, cheCluster.Status.CheClusterRunning)
	}
}

func checkPort(actualPort corev1.ServicePort, expectedName string, expectedPort int32, t *testing.T) {
	if actualPort.Name != expectedName || actualPort.Port != expectedPort {
		t.Errorf("expected port name:`%s` port:`%d`, actual name:`%s` port:`%d`",
			expectedName, expectedPort, actualPort.Name, actualPort.Port)
	}
}
