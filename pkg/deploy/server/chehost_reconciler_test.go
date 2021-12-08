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
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestSyncService(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	ctx := &deploy.DeployContext{
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

	server := NewCheHostReconciler()
	done, err := server.syncCheService(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	service := &corev1.Service{}
	done, err = deploy.Get(ctx, types.NamespacedName{Name: deploy.CheServiceName, Namespace: "eclipse-che"}, service)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.Equal(t, service.Spec.Ports[0].Name, "http")
	assert.Equal(t, service.Spec.Ports[0].Port, int32(8080))
	assert.Equal(t, service.Spec.Ports[1].Name, "metrics")
	assert.Equal(t, service.Spec.Ports[1].Port, deploy.DefaultCheMetricsPort)
	assert.Equal(t, service.Spec.Ports[2].Name, "debug")
	assert.Equal(t, service.Spec.Ports[2].Port, deploy.DefaultCheDebugPort)
}

func TestConfiguringLabelsForRoutes(t *testing.T) {
	util.IsOpenShift = true

	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      os.Getenv("CHE_FLAVOR"),
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheServerRoute: orgv1.RouteCustomSettings{
					Labels: "route=one",
				},
			},
		},
		Status: orgv1.CheClusterStatus{},
	}

	ctx := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})

	server := NewCheHostReconciler()
	done, err := server.exposeCheEndpoint(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	route := &routev1.Route{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: getComponentName(ctx), Namespace: "eclipse-che"}, route)
	assert.Nil(t, err)
	assert.Equal(t, route.ObjectMeta.Labels["route"], "one")
}
