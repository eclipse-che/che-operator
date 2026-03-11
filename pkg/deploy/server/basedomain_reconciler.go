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
	"fmt"
	"strings"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
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
			items := strings.SplitAfterN(ctx.CheHost, ".", 2)
			if len(items) != 2 {
				return reconcile.Result{}, false, fmt.Errorf("unable to detect base domain")
			}
			baseDomain = items[1]
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
