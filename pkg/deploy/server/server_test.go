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
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
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
				Name:      "eclipse-che",
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
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	server := NewServer(deployContext)
	done, err := server.syncService()
	if !done {
		if err != nil {
			t.Fatalf("Failed to sync service, error: %v", err)
		} else {
			t.Fatalf("Failed to sync service")
		}
	}

	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultCheFlavor(deployContext.CheCluster), Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Failed to get service, error: %v", err)
	}

	checkPort(service.Spec.Ports[0], "http", 8080, t)
	checkPort(service.Spec.Ports[1], "debug", deploy.DefaultCheDebugPort, t)
	checkPort(service.Spec.Ports[2], "metrics", deploy.DefaultCheMetricsPort, t)
}

func checkPort(actualPort corev1.ServicePort, expectedName string, expectedPort int32, t *testing.T) {
	if actualPort.Name != expectedName || actualPort.Port != expectedPort {
		t.Errorf("expected port name:`%s` port:`%d`, actual name:`%s` port:`%d`",
			expectedName, expectedPort, actualPort.Name, actualPort.Port)
	}
}
