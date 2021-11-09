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
package pluginregistry

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestPluginRegistrySyncAll(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
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
	}

	util.IsOpenShift = true

	pluginregistry := NewPluginRegistry(deployContext)
	done, err := pluginregistry.SyncAll()
	if !done || err != nil {
		t.Fatalf("Failed to sync Plugin Registry: %v", err)
	}

	// check service
	service := &corev1.Service{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, service)
	if err != nil {
		t.Fatalf("Service not found: %v", err)
	}

	// check endpoint
	route := &routev1.Route{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, route)
	if err != nil {
		t.Fatalf("Route not found: %v", err)
	}

	// check configmap
	cm := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, cm)
	if err != nil {
		t.Fatalf("Config Map not found: %v", err)
	}

	// check deployment
	deployment := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, deployment)
	if err != nil {
		t.Fatalf("Deployment not found: %v", err)
	}

	if cheCluster.Status.PluginRegistryURL == "" {
		t.Fatalf("Status wasn't updated")
	}
}
