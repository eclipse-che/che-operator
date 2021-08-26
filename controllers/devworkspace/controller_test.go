package devworkspace

import (
	"context"
	"os"
	"testing"
	"time"

	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	checluster "github.com/eclipse-che/che-operator/api"
	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/sync"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/api/node/v1alpha1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/utils/pointer"

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
	utilruntime.Must(v1.AddToScheme(scheme))
	utilruntime.Must(dwo.AddToScheme(scheme))

	return scheme
}

func TestNoCustomResourceSharedWhenReconcilingNonExistent(t *testing.T) {
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

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	// there is nothing in our context, so the map should still be empty
	managers := GetCurrentCheClusterInstances()
	if len(managers) != 0 {
		t.Fatalf("There should have been no managers after a reconcile of a non-existent manager.")
	}

	// now add some manager and reconcile a non-existent one
	cl.Create(ctx, asV1(&v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName + "-not-me",
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: v2alpha1.CheClusterSpec{
			Gateway: v2alpha1.CheGatewaySpec{
				Host:    "over.the.rainbow",
				Enabled: pointer.BoolPtr(false),
			},
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain: "down.on.earth",
			},
		},
	}))

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	managers = GetCurrentCheClusterInstances()
	if len(managers) != 0 {
		t.Fatalf("There should have been no managers after a reconcile of a non-existent manager.")
	}
}

func TestAddsCustomResourceToSharedMapOnCreate(t *testing.T) {
	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	cl := fake.NewFakeClientWithScheme(scheme, asV1(&v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: v2alpha1.CheClusterSpec{
			Gateway: v2alpha1.CheGatewaySpec{
				Host:    "over.the.rainbow",
				Enabled: pointer.BoolPtr(false),
			},
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain: "down.on.earth",
			},
		},
	}))

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
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
	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, asV1(&v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: v2alpha1.CheClusterSpec{
			Gateway: v2alpha1.CheGatewaySpec{
				Enabled: pointer.BoolPtr(false),
				Host:    "over.the.rainbow",
			},
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain: "down.on.earth",
			},
		},
	}))

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
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

	if mgr.Spec.Gateway.Host != "over.the.rainbow" {
		t.Fatalf("Unexpected host value: expected: over.the.rainbow, actual: %s", mgr.Spec.Gateway.Host)
	}

	// now update the manager and reconcile again. See that the map contains the updated value
	mgrInCluster := v1.CheCluster{}
	cl.Get(context.TODO(), client.ObjectKey{Name: managerName, Namespace: ns}, &mgrInCluster)

	// to be able to update, we need to set the resource version
	mgr.SetResourceVersion(mgrInCluster.GetResourceVersion())

	mgr.Spec.Gateway.Host = "over.the.shoulder"
	err = cl.Update(context.TODO(), asV1(&mgr))
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

	if mgr.Spec.Gateway.Host != "over.the.rainbow" {
		t.Fatalf("Unexpected host value: expected: over.the.rainbow, actual: %s", mgr.Spec.Gateway.Host)
	}

	// now reconcile and see that the value in the map is now updated

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
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

	if mgr.Spec.Gateway.Host != "over.the.shoulder" {
		t.Fatalf("Unexpected host value: expected: over.the.shoulder, actual: %s", mgr.Spec.Gateway.Host)
	}
}

func TestRemovesCustomResourceFromSharedMapOnDelete(t *testing.T) {
	// clear the map before the test
	for k := range currentCheInstances {
		delete(currentCheInstances, k)
	}

	managerName := "che"
	ns := "default"
	scheme := createTestScheme()

	cl := fake.NewFakeClientWithScheme(scheme, asV1(&v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       managerName,
			Namespace:  ns,
			Finalizers: []string{FinalizerName},
		},
		Spec: v2alpha1.CheClusterSpec{
			Gateway: v2alpha1.CheGatewaySpec{
				Host:    "over.the.rainbow",
				Enabled: pointer.BoolPtr(false),
			},
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain: "down.on.earth",
			},
		},
	}))

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
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

	cl.Delete(context.TODO(), asV1(&mgr))

	// now reconcile and see that the value is no longer in the map

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
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
	managerName := "che"
	ns := "default"
	scheme := createTestScheme()
	ctx := context.TODO()
	cl := fake.NewFakeClientWithScheme(scheme,
		asV1(&v2alpha1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       managerName,
				Namespace:  ns,
				Finalizers: []string{FinalizerName},
			},
			Spec: v2alpha1.CheClusterSpec{
				Gateway: v2alpha1.CheGatewaySpec{
					Host: "over.the.rainbow",
				},
				WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
					BaseDomain: "down.on.earth",
				},
			},
		}),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ws1",
				Namespace: ns,
				Annotations: map[string]string{
					defaults.ConfigAnnotationCheManagerName:      managerName,
					defaults.ConfigAnnotationCheManagerNamespace: ns,
				},
				Labels: defaults.GetLabelsFromNames(managerName, "gateway-config"),
			},
		})

	reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

	_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	// check that the reconcile loop added the finalizer
	manager := v1.CheCluster{}
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
	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	manager = v1.CheCluster{}
	err = cl.Get(ctx, client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	if err != nil {
		t.Fatalf("Failed to obtain the manager from the fake client: %s", err)
	}

	if len(manager.Finalizers) != 1 {
		t.Fatalf("There should have been a finalizer on the manager after a failed finalization attempt")
	}

	if manager.Status.DevworkspaceStatus.Phase != v2alpha1.ClusterPhasePendingDeletion {
		t.Fatalf("Expected the manager to be in the pending deletion phase but it is: %s", manager.Status.DevworkspaceStatus.Phase)
	}
	if len(manager.Status.DevworkspaceStatus.Message) == 0 {
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

	_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: managerName, Namespace: ns}})
	if err != nil {
		t.Fatalf("Failed to reconcile che manager with error: %s", err)
	}

	manager = v1.CheCluster{}
	err = cl.Get(ctx, client.ObjectKey{Name: managerName, Namespace: ns}, &manager)
	if err != nil {
		t.Fatalf("Failed to obtain the manager from the fake client: %s", err)
	}

	if len(manager.Finalizers) != 0 {
		t.Fatalf("The finalizers should be cleared after the finalization success but there were still some: %d", len(manager.Finalizers))
	}
}

// This test should be removed if we are again in charge of gateway creation.
func TestExternalGatewayDetection(t *testing.T) {
	origFlavor := os.Getenv("CHE_FLAVOR")
	t.Cleanup(func() {
		os.Setenv("CHE_FLAVOR", origFlavor)
	})

	os.Setenv("CHE_FLAVOR", "test-che")

	scheme := createTestScheme()

	clusterName := "eclipse-che"
	ns := "default"

	v2cluster := &v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: ns,
		},
		Spec: v2alpha1.CheClusterSpec{
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain: "down.on.earth",
			},
		},
	}

	onKubernetes(func() {
		v1Cluster := asV1(v2cluster)

		cl := fake.NewFakeClientWithScheme(scheme,
			v1Cluster,
			&networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress",
					Namespace: ns,
					Labels:    deploy.GetLabels(v1Cluster, "test-che"),
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "ingress.host",
						},
					},
				},
			},
		)

		reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

		// first reconcile sets the finalizer, second reconcile actually finishes the process
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}

		persisted := v1.CheCluster{}
		if err := cl.Get(context.TODO(), types.NamespacedName{Name: clusterName, Namespace: ns}, &persisted); err != nil {
			t.Fatal(err)
		}

		if persisted.Status.DevworkspaceStatus.Phase != v2alpha1.ClusterPhaseActive {
			t.Fatalf("Unexpected cluster state: %v", persisted.Status.DevworkspaceStatus.Phase)
		}

		if persisted.Status.DevworkspaceStatus.GatewayHost != "ingress.host" {
			t.Fatalf("Unexpected gateway host: %v", persisted.Status.DevworkspaceStatus.GatewayHost)
		}
	})

	onOpenShift(func() {
		v1Cluster := asV1(v2cluster)

		cl := fake.NewFakeClientWithScheme(scheme,
			v1Cluster,
			&routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "route",
					Namespace: ns,
					Labels:    deploy.GetLabels(v1Cluster, "test-che"),
				},
				Spec: routev1.RouteSpec{
					Host: "route.host",
				},
			},
		)

		reconciler := CheClusterReconciler{client: cl, scheme: scheme, syncer: sync.New(cl, scheme)}

		// first reconcile sets the finalizer, second reconcile actually finishes the process
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: clusterName, Namespace: ns}})
		if err != nil {
			t.Fatalf("Failed to reconcile che manager with error: %s", err)
		}

		persisted := v1.CheCluster{}
		if err := cl.Get(context.TODO(), types.NamespacedName{Name: clusterName, Namespace: ns}, &persisted); err != nil {
			t.Fatal(err)
		}

		if persisted.Status.DevworkspaceStatus.Phase != v2alpha1.ClusterPhaseActive {
			t.Fatalf("Unexpected cluster state: %v", persisted.Status.DevworkspaceStatus.Phase)
		}

		if persisted.Status.DevworkspaceStatus.GatewayHost != "route.host" {
			t.Fatalf("Unexpected gateway host: %v", persisted.Status.DevworkspaceStatus.GatewayHost)
		}
	})
}

func asV1(v2Obj *v2alpha1.CheCluster) *v1.CheCluster {
	return checluster.AsV1(v2Obj)
}

func onKubernetes(f func()) {
	isOpenShift := util.IsOpenShift
	isOpenShift4 := util.IsOpenShift4

	util.IsOpenShift = false
	util.IsOpenShift4 = false

	f()

	util.IsOpenShift = isOpenShift
	util.IsOpenShift4 = isOpenShift4
}

func onOpenShift(f func()) {
	isOpenShift := util.IsOpenShift
	isOpenShift4 := util.IsOpenShift4

	util.IsOpenShift = true
	util.IsOpenShift4 = true

	f()

	util.IsOpenShift = isOpenShift
	util.IsOpenShift4 = isOpenShift4
}
