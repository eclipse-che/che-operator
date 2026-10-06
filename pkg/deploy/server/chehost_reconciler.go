//
// Copyright (c) 2019-2026 Red Hat, Inc.
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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	routev1 "github.com/openshift/api/route/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CheHostReconciler struct {
	reconciler.Reconcilable
}

func NewCheHostReconciler() *CheHostReconciler {
	return &CheHostReconciler{}
}

func (s *CheHostReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := s.syncCheService(cheCtx); err != nil {
		return reconcile.Result{}, false, err
	}

	cheHost, done, err := s.exposeCheEndpoint(cheCtx)
	if !done {
		return reconcile.Result{}, false, err
	}

	cheCtx.CheHost = cheHost

	return reconcile.Result{}, true, nil
}

func (s *CheHostReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	return true
}

func (s *CheHostReconciler) syncCheService(cheCtx *chetypes.CheContext) error {
	portName := []string{"http"}
	portNumber := []int32{constants.DefaultServerPort}

	if cheCtx.CheCluster.Spec.Components.Metrics.Enable {
		portName = append(portName, "metrics")
		portNumber = append(portNumber, constants.DefaultServerMetricsPort)
	}

	if cheCtx.CheCluster.Spec.Components.CheServer.Debug != nil && *cheCtx.CheCluster.Spec.Components.CheServer.Debug {
		portName = append(portName, "debug")
		portNumber = append(portNumber, constants.DefaultServerDebugPort)
	}

	spec := deploy.GetServiceSpec(cheCtx, deploy.CheServiceName, portName, portNumber, getComponentName())
	return deploy.SyncServiceSpecToCluster(cheCtx, spec)
}

func (s CheHostReconciler) exposeCheEndpoint(cheCtx *chetypes.CheContext) (string, bool, error) {
	if !infrastructure.IsOpenShift() {
		if _, err := deploy.SyncIngressToCluster(
			cheCtx,
			getComponentName(),
			"",
			gateway.GatewayServiceName,
			constants.DefaultServerPort,
			getComponentName()); err != nil {
			return "", false, err
		}

		ingress := &networking.Ingress{}
		exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
			cheCtx.Context,
			types.NamespacedName{Name: getComponentName(), Namespace: cheCtx.CheCluster.Namespace},
			ingress,
		)
		if !exists {
			return "", false, err
		}

		return ingress.Spec.Rules[0].Host, true, nil
	}

	if err := deploy.SyncRouteToCluster(
		cheCtx,
		getComponentName(),
		"/",
		gateway.GatewayServiceName,
		constants.DefaultServerPort,
		getComponentName()); err != nil {
		return "", false, err
	}

	route := &routev1.Route{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: getComponentName(), Namespace: cheCtx.CheCluster.Namespace},
		route,
	)
	if !exists {
		return "", false, err
	}

	return route.Spec.Host, true, nil
}
