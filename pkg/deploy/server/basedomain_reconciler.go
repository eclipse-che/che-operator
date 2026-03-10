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
	"context"
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type BaseDomainReconciler struct {
	reconciler.Reconcilable
}

func NewBaseDomainReconciler() *BaseDomainReconciler {
	return &BaseDomainReconciler{}
}

func (r *BaseDomainReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	baseDomain := utils.GetValue(
		ctx.CheCluster.Spec.Components.CheServer.ExtraProperties["CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX"],
		ctx.CheCluster.Spec.Networking.Domain, // must be set for Kubernetes, see CheClusterValidator
	)

	if baseDomain == "" {
		if infrastructure.IsOpenShift() {
			openshiftBaseDomain, err := r.detectOpenShiftRouteBaseDomain(ctx)
			if err != nil {
				return reconcile.Result{}, false, err
			}

			baseDomain = openshiftBaseDomain
		}
	}

	if baseDomain == "" {
		return reconcile.Result{}, false, fmt.Errorf("unable to detect base domain")
	}

	if ctx.CheCluster.Status.WorkspaceBaseDomain != baseDomain {
		ctx.CheCluster.Status.WorkspaceBaseDomain = baseDomain
		if err := deploy.UpdateCheCRStatus(ctx, "WorkspaceBaseDomain", baseDomain); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (r *BaseDomainReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

// Tries to autodetect the route base domain.
func (r *BaseDomainReconciler) detectOpenShiftRouteBaseDomain(ctx *chetypes.DeployContext) (string, error) {
	name := "devworkspace-che-test"
	testRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.CheCluster.Namespace,
			Name:      name,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Spec: routev1.RouteSpec{
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: name,
			},
		},
	}

	if err := ctx.ClusterAPI.ClientWrapper.Create(context.TODO(), testRoute); err != nil {
		return "", err
	}

	defer func() {
		_ = ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{Name: testRoute.Name, Namespace: testRoute.Namespace},
			&routev1.Route{})
	}()

	items := strings.SplitAfterN(testRoute.Spec.Host, ".", 2)
	return items[1], nil
}
