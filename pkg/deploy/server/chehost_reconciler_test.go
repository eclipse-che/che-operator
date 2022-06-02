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

	"k8s.io/utils/pointer"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestSyncService(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	ctx := &chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      defaults.GetCheFlavor(),
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					CheServer: chev2.CheServer{
						Debug: pointer.BoolPtr(true),
					},
					Metrics: chev2.ServerMetrics{
						Enable: true,
					},
				},
			},
		},
		ClusterAPI: chetypes.ClusterAPI{
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
	assert.Equal(t, service.Spec.Ports[1].Port, constants.DefaultServerMetricsPort)
	assert.Equal(t, service.Spec.Ports[2].Name, "debug")
	assert.Equal(t, service.Spec.Ports[2].Port, constants.DefaultServerDebugPort)
}

func TestConfiguringLabelsForRoutes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Labels: map[string]string{"route": "one"},
			},
		},
		Status: chev2.CheClusterStatus{},
	}

	ctx := test.GetDeployContext(cheCluster, []runtime.Object{})

	server := NewCheHostReconciler()
	_, done, err := server.exposeCheEndpoint(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	route := &routev1.Route{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: getComponentName(ctx), Namespace: "eclipse-che"}, route)
	assert.Nil(t, err)
	assert.Equal(t, route.ObjectMeta.Labels["route"], "one")
}

func TestCheHostReconciler(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
	}
	ctx := test.GetDeployContext(cheCluster, []runtime.Object{})

	cheHostReconciler := NewCheHostReconciler()
	_, done, err := cheHostReconciler.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: getComponentName(ctx), Namespace: "eclipse-che"}, &routev1.Route{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: deploy.CheServiceName, Namespace: "eclipse-che"}, &corev1.Service{}))
}
