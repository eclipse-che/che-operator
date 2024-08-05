//
// Copyright (c) 2019-2024 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package devfileregistry

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DevfileRegistryReconciler struct {
	deploy.Reconcilable
}

func NewDevfileRegistryReconciler() *DevfileRegistryReconciler {
	return &DevfileRegistryReconciler{}
}

func (d *DevfileRegistryReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	_, _ = deploy.DeleteNamespacedObject(ctx, constants.DevfileRegistryName, &corev1.Service{})
	_, _ = deploy.DeleteNamespacedObject(ctx, constants.DevfileRegistryName, &corev1.ConfigMap{})
	_, _ = deploy.DeleteNamespacedObject(ctx, gateway.GatewayConfigMapNamePrefix+constants.DevfileRegistryName, &corev1.ConfigMap{})
	_, _ = deploy.DeleteNamespacedObject(ctx, constants.DevfileRegistryName, &appsv1.Deployment{})

	if ctx.CheCluster.Status.DevfileRegistryURL != "" {
		ctx.CheCluster.Status.DevfileRegistryURL = ""
		_ = deploy.UpdateCheCRStatus(ctx, "DevfileRegistryURL", "")
	}

	return reconcile.Result{}, true, nil
}

func (d *DevfileRegistryReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
