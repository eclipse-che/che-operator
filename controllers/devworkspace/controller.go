//
// Copyright (c) 2019-2020 Red Hat, Inc.
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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	checluster "github.com/eclipse-che/che-operator/api"
	checlusterv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	datasync "github.com/eclipse-che/che-operator/controllers/devworkspace/sync"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
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
	log                 = ctrl.Log.WithName("che")
	currentCheInstances = map[client.ObjectKey]v2alpha1.CheCluster{}
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
func GetCurrentCheClusterInstances() map[client.ObjectKey]v2alpha1.CheCluster {
	cheInstancesAccess.Lock()
	defer cheInstancesAccess.Unlock()

	ret := map[client.ObjectKey]v2alpha1.CheCluster{}

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

	currentCheInstances = map[client.ObjectKey]v2alpha1.CheCluster{}
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
		For(&checlusterv1.CheCluster{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbac.Role{}).
		Owns(&rbac.RoleBinding{})
	if infrastructure.IsOpenShift() {
		bld.Owns(&routev1.Route{})
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
	currentV1 := &checlusterv1.CheCluster{}
	err := r.client.Get(ctx, req.NamespacedName, currentV1)
	if err != nil {
		if errors.IsNotFound(err) {
			// Ok, our current router disappeared...
			return ctrl.Result{}, nil
		}
		// other error - let's requeue
		return ctrl.Result{}, err
	}

	current := checluster.AsV2alpha1(currentV1)

	if current.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.finalize(ctx, current, currentV1)
	}

	disabledMessage := ""
	switch GetDevworkspaceState(r.scheme, current) {
	case DevworkspaceStateNotPresent:
		disabledMessage = "Devworkspace CRDs are not installed"
	case DevworkspaceStateDisabled:
		disabledMessage = "Devworkspace Che is disabled"
	}

	if disabledMessage != "" {
		res, err := r.updateStatus(ctx, current, currentV1, nil, current.Status.GatewayHost, current.Status.WorkspaceBaseDomain, v2alpha1.ClusterPhaseInactive, disabledMessage)
		if err != nil {
			return res, err
		}

		currentV1 = &checlusterv1.CheCluster{}
		_ = r.client.Get(ctx, req.NamespacedName, currentV1)

		return res, nil
	}

	finalizerUpdated, err := r.ensureFinalizer(ctx, current)
	if err != nil {
		log.Info("Failed to set a finalizer", "object", req.String())
		return ctrl.Result{}, err
	} else if finalizerUpdated {
		// we've updated the object with a new finalizer, so we will enter another reconciliation loop shortly
		// we don't add the manager into the shared map just yet, because we have actually not reconciled it fully.
		return ctrl.Result{}, nil
	}

	// validate the CR
	err = r.validate(current)
	if err != nil {
		log.Info("validation errors", "errors", err.Error())
		res, err := r.updateStatus(ctx, current, currentV1, nil, current.Status.GatewayHost, current.Status.WorkspaceBaseDomain, v2alpha1.ClusterPhaseInactive, err.Error())
		if err != nil {
			return res, err
		}

		return res, nil
	}

	// now, finally, the actual reconciliation
	var changed bool
	var host string

	// We are no longer in charge of the gateway, leaving the responsibility for managing it on the che-operator.
	// But we need to detect the hostname on which the gateway is exposed so that the rest of our subsystems work.
	host, err = r.detectCheHost(ctx, currentV1)
	if err != nil {
		return ctrl.Result{}, err
	}

	// setting changed to false, because we jump from inactive directly to established, because we are no longer in
	// control of gateway creation
	changed = false

	workspaceBaseDomain := current.Spec.WorkspaceDomainEndpoints.BaseDomain

	if workspaceBaseDomain == "" {
		workspaceBaseDomain, err = r.detectOpenShiftRouteBaseDomain(current)
		if err != nil {
			return ctrl.Result{}, err
		}

		if workspaceBaseDomain == "" {
			res, err := r.updateStatus(ctx, current, currentV1, nil, current.Status.GatewayHost, current.Status.WorkspaceBaseDomain, v2alpha1.ClusterPhaseInactive, "Could not auto-detect the workspaceBaseDomain. Please set it explicitly in the spec.")
			if err != nil {
				return res, err
			}

			return res, nil
		}
	}

	res, err := r.updateStatus(ctx, current, currentV1, &changed, host, workspaceBaseDomain, v2alpha1.ClusterPhaseActive, "")

	if err != nil {
		return res, err
	}

	// everything went fine and the manager exists, put it back in the shared map
	currentCheInstances[req.NamespacedName] = *current

	return res, nil
}

func (r *CheClusterReconciler) updateStatus(ctx context.Context, cluster *v2alpha1.CheCluster, v1Cluster *checlusterv1.CheCluster, changed *bool, host string, workspaceDomain string, phase v2alpha1.ClusterPhase, phaseMessage string) (ctrl.Result, error) {
	currentPhase := cluster.Status.GatewayPhase

	if changed != nil {
		if !cluster.Spec.Gateway.IsEnabled() {
			cluster.Status.GatewayPhase = v2alpha1.GatewayPhaseInactive
		} else if *changed {
			cluster.Status.GatewayPhase = v2alpha1.GatewayPhaseInitializing
		} else {
			cluster.Status.GatewayPhase = v2alpha1.GatewayPhaseEstablished
		}
	}

	cluster.Status.GatewayHost = host
	cluster.Status.WorkspaceBaseDomain = workspaceDomain

	// set this unconditionally, because the only other value is set using the finalizer
	cluster.Status.Phase = phase
	cluster.Status.Message = phaseMessage

	var err error
	if !reflect.DeepEqual(v1Cluster.Status.DevworkspaceStatus, cluster.Status) {
		v1Cluster.Status.DevworkspaceStatus = cluster.Status
		err = r.client.Status().Update(ctx, v1Cluster)
	}

	requeue := cluster.Spec.IsEnabled() && (currentPhase == v2alpha1.GatewayPhaseInitializing ||
		cluster.Status.Phase != v2alpha1.ClusterPhaseActive)

	return ctrl.Result{Requeue: requeue}, err
}

func (r *CheClusterReconciler) validate(cluster *v2alpha1.CheCluster) error {
	validationErrors := []string{}

	if !infrastructure.IsOpenShift() {
		// The validation error messages must correspond to the storage version of the resource, which is currently
		// v1...
		if cluster.Spec.WorkspaceDomainEndpoints.BaseDomain == "" {
			validationErrors = append(validationErrors, "spec.k8s.ingressDomain must be specified")
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

func (r *CheClusterReconciler) finalize(ctx context.Context, cluster *v2alpha1.CheCluster, v1Cluster *checlusterv1.CheCluster) (err error) {
	err = r.gatewayConfigFinalize(ctx, cluster)

	if err == nil {
		finalizers := []string{}
		for i := range cluster.Finalizers {
			if cluster.Finalizers[i] != FinalizerName {
				finalizers = append(finalizers, cluster.Finalizers[i])
			}
		}

		cluster.Finalizers = finalizers

		err = r.client.Update(ctx, checluster.AsV1(cluster))
	} else {
		cluster.Status.Phase = v2alpha1.ClusterPhasePendingDeletion
		cluster.Status.Message = fmt.Sprintf("Finalization has failed: %s", err.Error())

		v1Cluster.Status.DevworkspaceStatus = cluster.Status
		err = r.client.Status().Update(ctx, v1Cluster)
	}

	return err
}

func (r *CheClusterReconciler) ensureFinalizer(ctx context.Context, cluster *v2alpha1.CheCluster) (updated bool, err error) {

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
		err = r.client.Update(ctx, checluster.AsV1(cluster))
	}

	return needsUpdate, err
}

// Tries to autodetect the route base domain.
func (r *CheClusterReconciler) detectOpenShiftRouteBaseDomain(cluster *v2alpha1.CheCluster) (string, error) {
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

func (r *CheClusterReconciler) detectCheHost(ctx context.Context, cluster *checlusterv1.CheCluster) (string, error) {
	host := cluster.Spec.Server.CheHost

	if host == "" {
		expectedLabels := deploy.GetLabels(cluster, deploy.DefaultCheFlavor(cluster))
		lbls := labels.SelectorFromSet(expectedLabels)

		if util.IsOpenShift {
			list := routev1.RouteList{}
			err := r.client.List(ctx, &list, &client.ListOptions{
				Namespace:     cluster.Namespace,
				LabelSelector: lbls,
			})

			if err != nil {
				return "", err
			}

			if len(list.Items) == 0 {
				return "", fmt.Errorf("expecting exactly 1 route to match Che gateway labels but found %d", len(list.Items))
			}

			host = list.Items[0].Spec.Host
		} else {
			list := networkingv1.IngressList{}
			err := r.client.List(ctx, &list, &client.ListOptions{
				Namespace:     cluster.Namespace,
				LabelSelector: lbls,
			})

			if err != nil {
				return "", err
			}

			if len(list.Items) == 0 {
				return "", fmt.Errorf("expecting exactly 1 ingress to match Che gateway labels but found %d", len(list.Items))
			}

			if len(list.Items[0].Spec.Rules) != 1 {
				return "", fmt.Errorf("expecting exactly 1 rule on the Che gateway ingress but found %d. This is a bug", len(list.Items[0].Spec.Rules))
			}

			host = list.Items[0].Spec.Rules[0].Host
		}
	}

	return host, nil
}

// Checks that there are no devworkspace configurations for the gateway (which would mean running devworkspaces).
// If there are some, an error is returned.
func (r *CheClusterReconciler) gatewayConfigFinalize(ctx context.Context, cluster *v2alpha1.CheCluster) error {
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
