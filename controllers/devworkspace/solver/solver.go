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

package solver

import (
	"fmt"
	"time"

	"github.com/devfile/devworkspace-operator/pkg/constants"

	controllerv1alpha1 "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	controller "github.com/eclipse-che/che-operator/controllers/devworkspace"
	"github.com/eclipse-che/che-operator/controllers/devworkspace/defaults"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	logger = ctrl.Log.WithName("solver")
)

// CheRoutingSolver is a struct representing the routing solver for Che specific routing of devworkspaces
type CheRoutingSolver struct {
	client client.Client
	scheme *runtime.Scheme
}

// Magic to ensure we get compile time error right here if our struct doesn't support the interface.
var _ solvers.RoutingSolverGetter = (*CheRouterGetter)(nil)
var _ solvers.RoutingSolver = (*CheRoutingSolver)(nil)

// CheRouterGetter negotiates the solver with the calling code
type CheRouterGetter struct {
	scheme *runtime.Scheme
}

// Getter creates a new CheRouterGetter
func Getter(scheme *runtime.Scheme) *CheRouterGetter {
	return &CheRouterGetter{
		scheme: scheme,
	}
}

func (g *CheRouterGetter) HasSolver(routingClass controllerv1alpha1.DevWorkspaceRoutingClass) bool {
	return isSupported(routingClass)
}

func (g *CheRouterGetter) GetSolver(client client.Client, routingClass controllerv1alpha1.DevWorkspaceRoutingClass) (solver solvers.RoutingSolver, err error) {
	if !isSupported(routingClass) {
		return nil, solvers.RoutingNotSupported
	}
	return &CheRoutingSolver{client: client, scheme: g.scheme}, nil
}

func (g *CheRouterGetter) SetupControllerManager(mgr *builder.Builder) error {

	// We want to watch configmaps and re-map the reconcile on the devworkspace routing, if possible
	// This way we can react on changes of the gateway configmap changes by re-reconciling the corresponding
	// devworkspace routing and thus keeping the devworkspace routing in a functional state
	// TODO is this going to be performant enough in a big cluster with very many configmaps?
	mgr.Watches(&source.Kind{Type: &corev1.ConfigMap{}}, handler.EnqueueRequestsFromMapFunc(func(mo client.Object) []reconcile.Request {
		applicable, key := isGatewayWorkspaceConfig(mo)

		if applicable {
			// cool, we can trigger the reconcile of the routing so that we can update the configmap that has just changed under our hands
			return []reconcile.Request{
				{
					NamespacedName: key,
				},
			}
		} else {
			return []reconcile.Request{}
		}
	}))

	return nil
}

func isGatewayWorkspaceConfig(obj client.Object) (bool, types.NamespacedName) {
	workspaceID := obj.GetLabels()[constants.DevWorkspaceIDLabel]
	objectName := obj.GetName()

	// bail out quickly if we're not dealing with a configmap with an expected name
	if objectName != defaults.GetGatewayWorkspaceConfigMapName(workspaceID) {
		return false, types.NamespacedName{}
	}

	routingName := obj.GetAnnotations()[defaults.ConfigAnnotationDevWorkspaceRoutingName]
	routingNamespace := obj.GetAnnotations()[defaults.ConfigAnnotationDevWorkspaceRoutingNamespace]

	// if there is no annotation for the routing, we're out of luck.. this should not happen though
	if routingName == "" {
		return false, types.NamespacedName{}
	}

	// cool, we found a configmap belonging to a concrete devworkspace routing
	return true, types.NamespacedName{Name: routingName, Namespace: routingNamespace}
}

func (c *CheRoutingSolver) FinalizerRequired(routing *controllerv1alpha1.DevWorkspaceRouting) bool {
	return true
}

func (c *CheRoutingSolver) Finalize(routing *controllerv1alpha1.DevWorkspaceRouting) error {
	cheManager, err := cheManagerOfRouting(routing)
	if err != nil {
		return err
	}

	return c.cheRoutingFinalize(cheManager, routing)
}

// GetSpecObjects constructs cluster routing objects which should be applied on the cluster
func (c *CheRoutingSolver) GetSpecObjects(routing *controllerv1alpha1.DevWorkspaceRouting, workspaceMeta solvers.DevWorkspaceMetadata) (solvers.RoutingObjects, error) {
	cheManager, err := cheManagerOfRouting(routing)
	if err != nil {
		return solvers.RoutingObjects{}, err
	}

	return c.cheSpecObjects(cheManager, routing, workspaceMeta)
}

// GetExposedEndpoints retreives the URL for each endpoint in a devfile spec from a set of RoutingObjects.
// Returns is a map from component ids (as defined in the devfile) to the list of endpoints for that component
// Return value "ready" specifies if all endpoints are resolved on the cluster; if false it is necessary to retry, as
// URLs will be undefined.
func (c *CheRoutingSolver) GetExposedEndpoints(endpoints map[string]controllerv1alpha1.EndpointList, routingObj solvers.RoutingObjects) (exposedEndpoints map[string]controllerv1alpha1.ExposedEndpointList, ready bool, err error) {
	if len(routingObj.Services) == 0 {
		return map[string]controllerv1alpha1.ExposedEndpointList{}, true, nil
	}

	managerName := routingObj.Services[0].Annotations[defaults.ConfigAnnotationCheManagerName]
	managerNamespace := routingObj.Services[0].Annotations[defaults.ConfigAnnotationCheManagerNamespace]
	workspaceID := routingObj.Services[0].Labels[constants.DevWorkspaceIDLabel]

	manager, err := findCheManager(client.ObjectKey{Name: managerName, Namespace: managerNamespace})
	if err != nil {
		return nil, false, err
	}

	return c.cheExposedEndpoints(manager, workspaceID, endpoints, routingObj)
}

func isSupported(routingClass controllerv1alpha1.DevWorkspaceRoutingClass) bool {
	return routingClass == "che"
}

func cheManagerOfRouting(routing *controllerv1alpha1.DevWorkspaceRouting) (*v2alpha1.CheCluster, error) {
	cheName := routing.Annotations[defaults.ConfigAnnotationCheManagerName]
	cheNamespace := routing.Annotations[defaults.ConfigAnnotationCheManagerNamespace]

	return findCheManager(client.ObjectKey{Name: cheName, Namespace: cheNamespace})
}

func findCheManager(cheManagerKey client.ObjectKey) (*v2alpha1.CheCluster, error) {
	managers := controller.GetCurrentCheClusterInstances()
	if len(managers) == 0 {
		// the CheManager has not been reconciled yet, so let's wait a bit
		return &v2alpha1.CheCluster{}, &solvers.RoutingNotReady{Retry: 1 * time.Second}
	}

	if len(cheManagerKey.Name) == 0 {
		if len(managers) > 1 {
			return &v2alpha1.CheCluster{}, &solvers.RoutingInvalid{Reason: fmt.Sprintf("the routing does not specify any Che manager in its configuration but there are %d Che managers in the cluster", len(managers))}
		}
		for _, m := range managers {
			return &m, nil
		}

	}

	if m, ok := managers[cheManagerKey]; ok {
		return &m, nil
	}

	logger.Info("Routing requires a non-existing che manager. Retrying in 10 seconds.", "key", cheManagerKey)

	return &v2alpha1.CheCluster{}, &solvers.RoutingNotReady{Retry: 10 * time.Second}
}
