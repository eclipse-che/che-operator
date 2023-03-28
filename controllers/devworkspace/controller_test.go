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

package devworkspace

import (
	"context"
	"os"
	"testing"
	"time"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"

	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	devworkspacedefaults "github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/sync"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/api/node/v1alpha1"
	rbac "k8s.io/api/rbac/v1"

	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func createTestScheme() *runtime.Scheme {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(chev2.AddToScheme(scheme))
	utilruntime.Must(dwo.AddToScheme(scheme))

	return scheme
}

func TestNoCustomResourceSharedWhenReconcilingNonExistent(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	cl := fake.NewFakeClientWithScheme(scheme)

	ctx := context.TODO()

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

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
	cl.Create(ctx, &chev2.CheCluster{
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
	scheme := createTestScheme()
	cl := fake.NewFakeClientWithScheme(scheme, &chev2.CheCluster{
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
	})

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

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
	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, &chev2.CheCluster{
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
	})

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

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
	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, &chev2.CheCluster{
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
	})

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

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
	scheme := createTestScheme()
	ctx := context.TODO()
	cl := fake.NewFakeClientWithScheme(scheme,
		&chev2.CheCluster{
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
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ws1",
				Namespace: ns,
				Annotations: map[string]string{
					devworkspacedefaults.ConfigAnnotationCheManagerName:      managerName,
					devworkspacedefaults.ConfigAnnotationCheManagerNamespace: ns,
				},
				Labels: devworkspacedefaults.GetLabelsFromNames(managerName, "gateway-config"),
			},
		})

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	// check that the reconcile loop added the finalizer
	manager := chev2.CheCluster{}
	err = cl.Get(ctx, client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	if err != nil {
		t.Fatalf("Failed to obtain the manager from the fake client: %s", err)
	}

	if len(manager.Finalizers) != 1 {
		t.Fatalf("Expected a single finalizer on the manager but found: %d", len(manager.Finalizers))
	}

	if manager.Finalizers[0] != FinalizerName {
		t.Fatalf("Expected a finalizer called %s but got %s", FinalizerName, manager.Finalizers[0])
	}

	// try to delete the manager and check that the configmap disallows that and that the status of the manager is updated
	manager.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	err = cl.Update(ctx, &manager)
	if err != nil {
		t.Fatalf("Failed to update the manager in the fake client: %s", err)
	}
	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	manager = chev2.CheCluster{}
	err = cl.Get(ctx, client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	if err != nil {
		t.Fatalf("Failed to obtain the manager from the fake client: %s", err)
	}

	if len(manager.Finalizers) != 1 {
		t.Fatalf("There should have been a finalizer on the manager after a failed finalization attempt")
	}

	if manager.Status.ChePhase != chev2.ClusterPhasePendingDeletion {
		t.Fatalf("Expected the manager to be in the pending deletion phase but it is: %s", manager.Status.ChePhase)
	}
	if len(manager.Status.Message) == 0 {
		t.Fatalf("Expected an non-empty message about the failed finalization in the manager status")
	}

	// now remove the config map and check that the finalization proceeds
	err = cl.Delete(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ws1",
			Namespace: ns,
		},
	})
	if err != nil {
		t.Fatalf("Failed to delete the test configmap: %s", err)
	}

	_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	manager = chev2.CheCluster{}
	err = cl.Get(ctx, client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	if err == nil || !k8sErrors.IsNotFound(err) {
		t.Fatalf("Failed to obtain the manager from the fake client: %s", err)
	}
}

// This test should be removed if we are again in charge of gateway creation.
func TestExternalGatewayDetection(t *testing.T) {
	origFlavor := defaults.GetCheFlavor()
	t.Cleanup(func() {
		os.Setenv("CHE_FLAVOR", origFlavor)
	})

	os.Setenv("CHE_FLAVOR", "test-che")

	scheme := createTestScheme()

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
		cl := fake.NewFakeClientWithScheme(scheme, cluster)

		reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

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
		cl := fake.NewFakeClientWithScheme(scheme, cluster)

		reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

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
