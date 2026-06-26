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

package openvsx_server

import (
	"context"

	_ "embed"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXServerReconciler struct {
	reconciler.Reconcilable
	extensionsVersion string
}

var (
	//go:embed application.yml
	applicationConfig string

	logger = ctrl.Log.WithName(constants.OpenVSXServerComponentName)
)

func NewOpenVSXServerReconciler() *OpenVSXServerReconciler {
	return &OpenVSXServerReconciler{
		extensionsVersion: "",
	}
}

func (r *OpenVSXServerReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		deleteResources(ctx)
		return reconcile.Result{}, true, nil
	}

	err := r.syncConfigMap(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Config %w", err)
	}

	err = r.syncPVC(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync PVC: %w", err)
	}

	done, err := r.syncDeployment(ctx)
	if !done {
		if err != nil {
			err = fmt.Errorf("failed to sync Deployment %w", err)
		}
		return reconcile.Result{}, false, err
	}

	err = r.syncService(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Service %w", err)
	}

	err = r.syncIngress(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Ingress: %w", err)
	}

	err = r.syncDefaultExtensionsConfig(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Extensions Config: %w", err)
	}

	err = r.syncExtensions(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Extensions %w", err)
	}

	return reconcile.Result{}, true, nil
}

func deleteResources(ctx *chetypes.DeployContext) {
	cw := ctx.ClusterAPI.ClientWrapper

	objKey := types.NamespacedName{
		Name:      constants.OpenVSXServerComponentName,
		Namespace: ctx.CheCluster.Namespace,
	}

	err := cw.DeleteByKeyIgnoreNotFound(context.TODO(), objKey, &networkingv1.Ingress{})
	if err != nil {
		logger.Error(err, "failed to delete Ingress", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objKey, &corev1.Service{})
	if err != nil {
		logger.Error(err, "Failed to delete Service", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objKey, &appsv1.Deployment{})
	if err != nil {
		logger.Error(err, "Failed to delete Deployment", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objKey, &corev1.PersistentVolumeClaim{})
	if err != nil {
		logger.Error(err, "Failed to delete PVC", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(context.TODO(), objKey, &corev1.ConfigMap{})
	if err != nil {
		logger.Error(err, "Failed to delete ConfigMap", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXServerExtensionPublishJobName,
			Namespace: ctx.CheCluster.Namespace,
		},
		&batchv1.Job{},
	)
	if err != nil {
		logger.Error(err, "Failed to delete Job", "Name", constants.OpenVSXServerExtensionPublishJobName)
	}

	err = cw.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXServerExtensionsConfigMapName,
			Namespace: ctx.CheCluster.Namespace,
		},
		&corev1.ConfigMap{},
	)
	if err != nil {
		logger.Error(err, "Failed to delete ConfigMap", "Name", constants.OpenVSXServerExtensionsConfigMapName)
	}

	if ctx.CheCluster.Status.OpenVSXURL != "" {
		ctx.CheCluster.Status.OpenVSXURL = ""

		if err = deploy.UpdateCheCRStatus(ctx, "status: OpenVSXURL", ""); err != nil {
			logger.Error(err, "Failed to update status for OpenVSXURL")
		}
	}
}

func (r *OpenVSXServerReconciler) Finalize(_ *chetypes.DeployContext) bool {
	return true
}
