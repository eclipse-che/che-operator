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
package deploy

import (
	"context"
	"time"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	console "github.com/openshift/api/console/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestReconcileConsoleLink(t *testing.T) {
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

	scheme := scheme.Scheme
	scheme.AddKnownTypes(orgv1.SchemeBuilder.GroupVersion, &orgv1.CheCluster{})
	scheme.AddKnownTypes(console.GroupVersion, &console.ConsoleLink{})
	cli := fake.NewFakeClientWithScheme(scheme, cheCluster)
	clientSet := fakeclientset.NewSimpleClientset()
	fakeDiscovery, _ := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
	fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: ConsoleLinksResourceName},
			},
		},
	}

	util.IsOpenShift4 = true
	deployContext := &DeployContext{
		CheCluster: cheCluster,
		ClusterAPI: ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme,
			DiscoveryClient:  fakeDiscovery,
		},
	}

	done, err := ReconcileConsoleLink(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to reconcile consolelink: %v", err)
	}

	// check consolelink object existence
	consoleLink := &console.ConsoleLink{}
	exists, err := Get(deployContext, types.NamespacedName{Name: DefaultConsoleLinkName()}, consoleLink)
	if !exists || err != nil {
		t.Fatalf("Failed to get consolelink: %v", err)
	}

	// check finalizer
	c := &orgv1.CheCluster{}
	err = cli.Get(context.TODO(), types.NamespacedName{Namespace: "eclipse-che", Name: "eclipse-che"}, c)
	if err != nil {
		t.Fatalf("Failed to get checluster: %v", err)
	}
	if !util.ContainsString(c.ObjectMeta.Finalizers, ConsoleLinkFinalizerName) {
		t.Fatalf("Failed to add finalizer")
	}

	// Initialize DeletionTimestamp => checluster is being deleted
	cheCluster.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	err = ReconcileConsoleLinkFinalizer(deployContext)
	if err != nil {
		t.Fatalf("Failed to reconcile consolelink: %v", err)
	}

	// check consolelink object existence
	exists, err = Get(deployContext, types.NamespacedName{Name: DefaultConsoleLinkName()}, consoleLink)
	if exists || err != nil {
		t.Fatalf("Failed to remove consolelink")
	}

	// check finalizer
	c = &orgv1.CheCluster{}
	err = cli.Get(context.TODO(), types.NamespacedName{Namespace: "eclipse-che", Name: "eclipse-che"}, c)
	if !errors.IsNotFound(err) {
		t.Fatalf("Failed to get checluster: %v", err)
	}
}
