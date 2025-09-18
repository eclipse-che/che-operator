//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devworkspace

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	devworkspacedefaults "github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestNoCustomResourceSharedWhenReconcilingNonExistent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"

	ctx := test.NewCtxBuilder().Build()
	scheme := ctx.ClusterAPI.Scheme
	cl := ctx.ClusterAPI.Client

	reconciler := CheClusterReconciler{client: cl, scheme: scheme}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	// there is nothing in our context, so the map should still be empty
	managers := GetCurrentCheClusterInstances()
	if len(managers) != 0 {
		t.Fatalf("There should have been no managers after a reconcile of a non-existent manager.")
	}

	// now add some manager and reconcile a non-existent one
	cl.Create(context.TODO(), &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName + "-not-me",
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "over.the.rainbow",
			},
		},
		Status: chev2.CheClusterStatus{
			WorkspaceBaseDomain: "down.on.earth",
		},
	})

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentCheClusterInstances()
	if len(managers) != 0 {
		t.Fatalf("There should have been no managers after a reconcile of a non-existent manager.")
	}
}

func TestAddsCustomResourceToSharedMapOnCreate(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "over.the.rainbow",
				Domain:   "down.on.earth",
			},
		},
	}).Build()
	cl := ctx.ClusterAPI.Client
	scheme := ctx.ClusterAPI.Scheme

	reconciler := CheClusterReconciler{client: cl, scheme: scheme}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers := GetCurrentCheClusterInstances()
	if len(managers) != 1 {
		t.Fatalf("There should have been exactly 1 manager after a reconcile but there is %d.", len(managers))
	}

	mgr, ok := managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}
}

func TestUpdatesCustomResourceInSharedMapOnUpdate(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "over.the.rainbow",
				Domain:   "down.on.earth",
			},
		},
	}).Build()
	scheme := ctx.ClusterAPI.Scheme
	cl := ctx.ClusterAPI.Client

	reconciler := CheClusterReconciler{client: cl, scheme: scheme}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers := GetCurrentCheClusterInstances()
	if len(managers) != 1 {
		t.Fatalf("There should have been exactly 1 manager after a reconcile but there is %d.", len(managers))
	}

	mgr, ok := managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	if mgr.GetCheHost() != "over.the.rainbow" {
		t.Fatalf("Unexpected host value: expected: over.the.rainbow, actual: %s", mgr.GetCheHost())
	}

	// now update the manager and reconcile again. See that the map contains the updated value
	mgrInCluster := chev2.CheCluster{}
	cl.Get(context.TODO(), client.ObjectKey{Name: managerName, Namespace: ns}, &mgrInCluster)

	// to be able to update, we need to set the resource version
	mgr.SetResourceVersion(mgrInCluster.GetResourceVersion())

	mgr.Spec.Networking.Hostname = "over.the.shoulder"
	err = cl.Update(context.TODO(), &mgr)
	if err != nil {
		t.Fatalf("Failed to update. Wat? %s", err)
	}

	// before the reconcile, the map still should containe the old value
	managers = GetCurrentCheClusterInstances()
	mgr, ok = managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	if mgr.Spec.Networking.Hostname != "over.the.rainbow" {
		t.Fatalf("Unexpected host value: expected: over.the.rainbow, actual: %s", mgr.Spec.Networking.Hostname)
	}

	// now reconcile and see that the value in the map is now updated

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentCheClusterInstances()
	mgr, ok = managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	if mgr.Spec.Networking.Hostname != "over.the.shoulder" {
		t.Fatalf("Unexpected host value: expected: over.the.shoulder, actual: %s", mgr.Spec.Networking.Hostname)
	}
}

func TestRemovesCustomResourceFromSharedMapOnDelete(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "over.the.rainbow",
				Domain:   "down.on.earth",
			},
		},
	}).Build()
	cl := ctx.ClusterAPI.Client
	scheme := ctx.ClusterAPI.Scheme

	reconciler := CheClusterReconciler{client: cl, scheme: scheme}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers := GetCurrentCheClusterInstances()
	if len(managers) != 1 {
		t.Fatalf("There should have been exactly 1 manager after a reconcile but there is %d.", len(managers))
	}

	mgr, ok := managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if !ok {
		t.Fatalf("The map of the current managers doesn't contain the expected one.")
	}

	if mgr.Name != managerName {
		t.Fatalf("Found a manager that we didn't reconcile. Curious (and buggy). We found %s but should have found %s", mgr.Name, managerName)
	}

	cl.Delete(context.TODO(), &mgr)

	// now reconcile and see that the value is no longer in the map

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentCheClusterInstances()
	_, ok = managers[types.NamespacedName{Name: managerName, Namespace: ns}]
	if ok {
		t.Fatalf("The map of the current managers should no longer contain the manager after it has been deleted.")
	}
}

func TestCustomResourceFinalization(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	managerName := "che"
	ns := "default"

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname: "over.the.rainbow",
				Domain:   "down.on.earth",
			},
		},
	}).WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ws1",
			Namespace: ns,
			Annotations: map[string]string{
				devworkspacedefaults.ConfigAnnotationCheManagerName:      managerName,
				devworkspacedefaults.ConfigAnnotationCheManagerNamespace: ns,
			},
			Labels: devworkspacedefaults.GetLabelsFromNames(managerName, "gateway-config"),
		},
	}).Build()
	cl := ctx.ClusterAPI.Client
	scheme := ctx.ClusterAPI.Scheme

	reconciler := CheClusterReconciler{client: cl, scheme: scheme}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	assert.NoError(t, err)

	// check that the reconcile loop added the finalizer
	manager := chev2.CheCluster{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manager.Finalizers))
	assert.Equal(t, FinalizerName, manager.Finalizers[0])

	// try to delete the manager and check that the configmap disallows that and that the status of the manager is updated
	err = cl.Delete(context.TODO(), &manager)
	assert.NoError(t, err)

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	assert.NoError(t, err)

	manager = chev2.CheCluster{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manager.Finalizers))
	assert.Equal(t, chev2.ClusterPhasePendingDeletion, string(manager.Status.ChePhase))
	assert.NotEqual(t, 0, len(manager.Status.Message))

	// now remove the config map and check that the finalization proceeds
	err = cl.Delete(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ws1",
			Namespace: ns,
		},
	})
	assert.NoError(t, err)

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	assert.NoError(t, err)

	manager = chev2.CheCluster{}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

// This test should be removed if we are again in charge of gateway creation.
func TestExternalGatewayDetection(t *testing.T) {
	origFlavor := defaults.GetCheFlavor()
	t.Cleanup(func() {
		os.Setenv("CHE_FLAVOR", origFlavor)
	})

	os.Setenv("CHE_FLAVOR", "test-che")

	clusterName := "eclipse-che"
	ns := "default"

	cluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: ns,
		},
		Status: chev2.CheClusterStatus{
			WorkspaceBaseDomain: "down.on.earth",
			CheURL:              "https://host",
		},
	}

	onKubernetes(func() {
		ctx := test.NewCtxBuilder().WithCheCluster(cluster).Build()
		cl := ctx.ClusterAPI.Client
		scheme := ctx.ClusterAPI.Scheme

		reconciler := CheClusterReconciler{client: cl, scheme: scheme}

		// first reconcile sets the finalizer, second reconcile actually finishes the process
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}

		persisted := chev2.CheCluster{}
		if err := cl.Get(context.TODO(), types.NamespacedName{Name: clusterName, Namespace: ns}, &persisted); err != nil {
			t.Fatal(err)
		}

		if persisted.GetCheHost() != "host" {
			t.Fatalf("Unexpected gateway host: %v", persisted.GetCheHost())
		}
	})

	onOpenShift(func() {
		ctx := test.NewCtxBuilder().WithCheCluster(cluster).Build()
		cl := ctx.ClusterAPI.Client
		scheme := ctx.ClusterAPI.Scheme

		reconciler := CheClusterReconciler{client: cl, scheme: scheme}

		// first reconcile sets the finalizer, second reconcile actually finishes the process
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}

		persisted := chev2.CheCluster{}
		if err := cl.Get(context.TODO(), types.NamespacedName{Name: clusterName, Namespace: ns}, &persisted); err != nil {
			t.Fatal(err)
		}

		if persisted.GetCheHost() != "host" {
			t.Fatalf("Unexpected gateway host: %v", persisted.GetCheHost())
		}
	})
}

func onKubernetes(f func()) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	f()
}

func onOpenShift(f func()) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)
	f()
}
