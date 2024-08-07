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
	"context"
	"encoding/json"

	v2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	logger = ctrl.Log.WithName("devfile-registry")
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
		var externalDevfileRegistries []v2.ExternalDevfileRegistry
		if err := json.Unmarshal([]byte(defaults.GetDevfileRegistryExternalDevfileRegistries()), &externalDevfileRegistries); err == nil {

			// Add default external devfile registries to the CheCluster CR
			for _, newRegistry := range externalDevfileRegistries {
				newRegistryAlreadyExists := false
				for _, existedRegistry := range ctx.CheCluster.Spec.Components.DevfileRegistry.ExternalDevfileRegistries {
					if existedRegistry.Url == newRegistry.Url {
						newRegistryAlreadyExists = true
						break
					}
				}

				if !newRegistryAlreadyExists {
					logger.Info("Adding external devfile registry to the CheCluster CR", "Url", newRegistry.Url)
					ctx.CheCluster.Spec.Components.DevfileRegistry.ExternalDevfileRegistries =
						append(ctx.CheCluster.Spec.Components.DevfileRegistry.ExternalDevfileRegistries, newRegistry)
				}
			}

			if err := ctx.ClusterAPI.Client.Update(context.TODO(), ctx.CheCluster); err != nil {
				return reconcile.Result{}, false, err
			}
		} else {
			logger.Error(
				err, "Failed to unmarshal environment variable",
				"key", "CHE_OPERATOR_DEFAULTS_DEVFILE_REGISTRY_EXTERNAL_DEVFILE_REGISTRIES",
				"value", defaults.GetDevfileRegistryExternalDevfileRegistries(),
			)
		}

		ctx.CheCluster.Status.DevfileRegistryURL = ""
		_ = deploy.UpdateCheCRStatus(ctx, "DevfileRegistryURL", "")
	}

	return reconcile.Result{}, true, nil
}

func (d *DevfileRegistryReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
