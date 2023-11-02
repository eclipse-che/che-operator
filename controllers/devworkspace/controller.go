// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"encoding/hex"
	stdErrors "errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	datasync "github.com/eclipse-che/che-operator/controllers/devworkspace/sync"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log                 = ctrl.Log.WithName("devworkspace-che")
	currentCheInstances = map[client.ObjectKey]chev2.CheCluster{}
	cheInstancesAccess  = sync.Mutex{}
)

const (
	// FinalizerName is the name of the finalizer put on the Che Cluster resources by the controller. Public for testing purposes.
	FinalizerName = "checluster.che.eclipse.org"
)

type CheClusterReconciler struct {
	client client.Client
	scheme *runtime.Scheme
	syncer datasync.Syncer
}

// GetCurrentCheClusterInstances returns a map of all che clusters (keyed by their namespaced name)
// the che cluster controller currently knows of. This returns any meaningful data
// only after reconciliation has taken place.
//
// If this method is called from another controller, it effectively couples that controller
// with the che manager controller. Such controller will therefore have to run in the same
// process as the che manager controller. On the other hand, using this method, and somehow
// tolerating its eventual consistency, makes the other controller more efficient such that
// it doesn't have to find the che managers in the cluster (which is what che manager reconciler
// is doing).
//
// If need be, this method can be replaced by a simply calling client.List to get all the che
// managers in the cluster.
func GetCurrentCheClusterInstances() map[client.ObjectKey]chev2.CheCluster {
	cheInstancesAccess.Lock()
	defer cheInstancesAccess.Unlock()

	ret := map[client.ObjectKey]chev2.CheCluster{}

	for k, v := range currentCheInstances {
		ret[k] = v
	}

	return ret
}

// CleanCheClusterInstancesForTest is a helper function for test code in other packages that needs
// to re-initialize the state of the checluster instance cache.
func CleanCheClusterInstancesForTest() {
	cheInstancesAccess.Lock()
	defer cheInstancesAccess.Unlock()

	currentCheInstances = map[client.ObjectKey]chev2.CheCluster{}
}

// New returns a new instance of the Che manager reconciler. This is mainly useful for
// testing because it doesn't set up any watches in the cluster, etc. For that use SetupWithManager.
func New(cl client.Client, scheme *runtime.Scheme) CheClusterReconciler {
	return CheClusterReconciler{
		client: cl,
		scheme: scheme,
		syncer: datasync.New(cl, scheme),
	}
}

func (r *CheClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.scheme = mgr.GetScheme()
	r.syncer = datasync.New(r.client, r.scheme)

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&chev2.CheCluster{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbac.Role{}).
		Owns(&rbac.RoleBinding{})
	if infrastructure.IsOpenShift() {
		bld.Owns(&routev1.Route{})
	} else {
		bld.Owns(&networkingv1.Ingress{})
	}
	return bld.Complete(r)
}

func (r *CheClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	cheInstancesAccess.Lock()
	defer cheInstancesAccess.Unlock()

	// remove the manager from the shared map for the time of the reconciliation
	// we'll add it back if it is successfully reconciled.
	// The access to the map is locked for the time of reconciliation so that outside
	// callers don't witness this intermediate state.
	delete(currentCheInstances, req.NamespacedName)

	// make sure we've checked we're in a valid state
	cluster := &chev2.CheCluster{}
	err := r.client.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// Ok, our current router disappeared...
			return ctrl.Result{}, nil
		}
		// other error - let's requeue
		return ctrl.Result{}, err
	}

	if cluster.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.finalize(ctx, cluster)
	}

	finalizerUpdated, err := r.ensureFinalizer(ctx, cluster)
	if err != nil {
		log.Info("Failed to set a finalizer", "object", req.String())
		return ctrl.Result{}, err
	} else if finalizerUpdated {
		// we've updated the object with a new finalizer, so we will enter another reconciliation loop shortly
		// we don't add the manager into the shared map just yet, because we have actually not reconciled it fully.
		return ctrl.Result{}, nil
	}

	// validate the CR
	err = r.validate(cluster)
	if err != nil {
		log.Info("validation errors", "errors", err.Error())
		res, err := r.updateStatus(ctx, cluster, nil, cluster.Status.WorkspaceBaseDomain, chev2.ClusterPhaseInactive, err.Error())
		if err != nil {
			return res, err
		}

		return res, nil
	}

	// now, finally, the actual reconciliation
	var changed bool

	// We are no longer in charge of the gateway, leaving the responsibility for managing it on the che-operator.
	// But we need to detect the hostname on which the gateway is exposed so that the rest of our subsystems work.
	if cluster.GetCheHost() == "" {
		// Wait some time in case the route is not ready yet
		return ctrl.Result{RequeueAfter: 2 * time.Second}, err
	}

	// setting changed to false, because we jump from inactive directly to established, because we are no longer in
	// control of gateway creation
	changed = false

	workspaceBaseDomain := cluster.Spec.Networking.Domain

	// to be compatible with CheCluster API v1
	routeDomain := cluster.Spec.Components.CheServer.ExtraProperties["CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX"]
	if routeDomain != "" {
		workspaceBaseDomain = routeDomain
	}

	if workspaceBaseDomain == "" {
		workspaceBaseDomain, err = r.detectOpenShiftRouteBaseDomain(cluster)
		if err != nil {
			return ctrl.Result{}, err
		}

		if workspaceBaseDomain == "" {
			res, err := r.updateStatus(ctx, cluster, nil, cluster.Status.WorkspaceBaseDomain, chev2.ClusterPhaseInactive, "Could not auto-detect the workspaceBaseDomain. Please set it explicitly in the spec.")
			if err != nil {
				return res, err
			}

			return res, nil
		}
	}

	res, err := r.updateStatus(ctx, cluster, &changed, workspaceBaseDomain, chev2.ClusterPhaseActive, "")

	if err != nil {
		return res, err
	}

	// everything went fine and the manager exists, put it back in the shared map
	currentCheInstances[req.NamespacedName] = *cluster

	return res, nil
}

func (r *CheClusterReconciler) updateStatus(ctx context.Context, cluster *chev2.CheCluster, changed *bool, workspaceDomain string, phase chev2.CheClusterPhase, phaseMessage string) (ctrl.Result, error) {
	currentPhase := cluster.Status.GatewayPhase

	if changed != nil {
		if *changed {
			cluster.Status.GatewayPhase = chev2.GatewayPhaseInitializing
		} else {
			cluster.Status.GatewayPhase = chev2.GatewayPhaseEstablished
		}
	}

	cluster.Status.WorkspaceBaseDomain = workspaceDomain
	err := r.client.Status().Update(ctx, cluster)

	requeue := currentPhase == chev2.GatewayPhaseInitializing
	return ctrl.Result{Requeue: requeue}, err
}

func (r *CheClusterReconciler) validate(cluster *chev2.CheCluster) error {
	validationErrors := []string{}

	if !infrastructure.IsOpenShift() {
		if cluster.Spec.Networking.Domain == "" {
			validationErrors = append(validationErrors, "spec.networking.domain must be specified")
		}
	}

	if len(validationErrors) > 0 {
		message := "The following validation errors were detected:\n"
		for _, m := range validationErrors {
			message += "- " + m + "\n"
		}

		return stdErrors.New(message)
	}

	return nil
}

func (r *CheClusterReconciler) finalize(ctx context.Context, cluster *chev2.CheCluster) (err error) {
	err = r.gatewayConfigFinalize(ctx, cluster)

	if err == nil {
		finalizers := []string{}
		for i := range cluster.Finalizers {
			if cluster.Finalizers[i] != FinalizerName {
				finalizers = append(finalizers, cluster.Finalizers[i])
			}
		}

		cluster.Finalizers = finalizers

		err = r.client.Update(ctx, cluster)
	} else {
		cluster.Status.ChePhase = chev2.ClusterPhasePendingDeletion
		cluster.Status.Message = fmt.Sprintf("Finalization has failed: %s", err.Error())
		err = r.client.Status().Update(ctx, cluster)
	}

	return err
}

func (r *CheClusterReconciler) ensureFinalizer(ctx context.Context, cluster *chev2.CheCluster) (updated bool, err error) {

	needsUpdate := true
	if cluster.Finalizers != nil {
		for i := range cluster.Finalizers {
			if cluster.Finalizers[i] == FinalizerName {
				needsUpdate = false
				break
			}
		}
	} else {
		cluster.Finalizers = []string{}
	}

	if needsUpdate {
		cluster.Finalizers = append(cluster.Finalizers, FinalizerName)
		err = r.client.Update(ctx, cluster)
	}

	return needsUpdate, err
}

// Tries to autodetect the route base domain.
func (r *CheClusterReconciler) detectOpenShiftRouteBaseDomain(cluster *chev2.CheCluster) (string, error) {
	if !infrastructure.IsOpenShift() {
		return "", nil
	}

	name := "devworkspace-che-test-" + randomSuffix(8)
	testRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      name,
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: name,
			},
		},
	}

	err := r.client.Create(context.TODO(), testRoute)
	if err != nil {
		return "", err
	}
	defer r.client.Delete(context.TODO(), testRoute)
	host := testRoute.Spec.Host

	prefixToRemove := name + "-" + cluster.Namespace + "."
	return strings.TrimPrefix(host, prefixToRemove), nil
}

func randomSuffix(length int) string {
	var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

	arr := make([]byte, (length+1)/2) // to make even-length array so that it is convertible to hex
	rnd.Read(arr)

	return hex.EncodeToString(arr)
}

// Checks that there are no devworkspace configurations for the gateway (which would mean running devworkspaces).
// If there are some, an error is returned.
func (r *CheClusterReconciler) gatewayConfigFinalize(ctx context.Context, cluster *chev2.CheCluster) error {
	// we need to stop the reconcile if there are devworkspaces handled by it.
	// we detect that by the presence of the gateway configmaps in the namespace of the manager
	list := corev1.ConfigMapList{}

	err := r.client.List(ctx, &list, &client.ListOptions{
		Namespace:     cluster.Namespace,
		LabelSelector: labels.SelectorFromSet(defaults.GetLabelsForComponent(cluster, "gateway-config")),
	})
	if err != nil {
		return err
	}

	workspaceCount := 0

	for _, c := range list.Items {
		if c.Annotations[defaults.ConfigAnnotationCheManagerName] == cluster.Name && c.Annotations[defaults.ConfigAnnotationCheManagerNamespace] == cluster.Namespace {
			workspaceCount++
		}
	}

	if workspaceCount > 0 {
		return fmt.Errorf("there are %d devworkspaces associated with this Che manager", workspaceCount)
	}

	return nil
}
