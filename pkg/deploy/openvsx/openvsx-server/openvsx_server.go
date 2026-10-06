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
	_ "embed"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXServerReconciler struct {
	reconciler.Reconcilable

	// extensionsVersion tracks the last synced ConfigMap version to avoid unnecessary Job churn.
	// Resets on operator restart, which is safe - the Job is idempotent.
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

func (r *OpenVSXServerReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if !cheCtx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		deleteResources(cheCtx)
		r.extensionsVersion = ""
		return reconcile.Result{}, true, nil
	}

	err := r.syncConfigMap(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Config %w", err)
	}

	err = r.syncPVC(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync PVC: %w", err)
	}

	done, err := r.syncDeployment(cheCtx)
	if !done {
		if err != nil {
			err = fmt.Errorf("failed to sync Deployment %w", err)
		}
		return reconcile.Result{}, false, err
	}

	err = r.syncService(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Service %w", err)
	}

	_, done, err = r.exposeEndpoint(cheCtx)
	if !done {
		if err != nil {
			err = fmt.Errorf("failed to expose endpoint: %w", err)
		}
		return reconcile.Result{}, false, err
	}

	err = r.syncOpenVSXURLStatus(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync OpenVSXURL status: %w", err)
	}

	err = r.syncDefaultExtensionsConfig(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Extensions Config: %w", err)
	}

	if err = r.syncExtensionUpdateCronJob(cheCtx); err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync extension update CronJob: %w", err)
	}

	if !r.isServerReady(cheCtx) {
		return reconcile.Result{}, false, nil
	}

	extensionsVersion, err := r.getExtensionsVersion(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to get Extensions Version: %w", err)
	}
	if extensionsVersion != r.extensionsVersion {
		done, err = r.syncExtensions(cheCtx)
		if !done {
			if err != nil {
				err = fmt.Errorf("failed to sync Extensions %w", err)
			}
			return reconcile.Result{}, false, err
		}

		r.extensionsVersion = extensionsVersion
	}

	return reconcile.Result{}, true, nil
}

func deleteResources(cheCtx *chetypes.CheContext) {
	cw := cheCtx.ClusterAPI.ClientWrapper

	objKey := types.NamespacedName{
		Name:      constants.OpenVSXServerComponentName,
		Namespace: cheCtx.CheCluster.Namespace,
	}

	err := cw.DeleteByKeyIgnoreNotFound(cheCtx.Context, objKey, &appsv1.Deployment{})
	if err != nil {
		logger.Error(err, "Failed to delete Deployment", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{
			Name:      constants.OpenVSXServerExtensionPublishJobName,
			Namespace: cheCtx.CheCluster.Namespace,
		},
		&batchv1.Job{},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)
	if err != nil {
		logger.Error(err, "Failed to delete Job", "Name", constants.OpenVSXServerExtensionPublishJobName)
	}

	err = deleteExtensionUpdateCronJob(cheCtx)
	if err != nil {
		logger.Error(err, "Failed to delete CronJob", "Name", constants.OpenVSXServerExtensionUpdateCronJobName)
	}

	err = cw.DeleteByKeyIgnoreNotFound(cheCtx.Context, objKey, &corev1.Service{})
	if err != nil {
		logger.Error(err, "Failed to delete Service", "Name", objKey.Name)
	}

	gatewayConfigKey := types.NamespacedName{
		Name:      gateway.GatewayConfigMapNamePrefix + constants.OpenVSXServerComponentName,
		Namespace: cheCtx.CheCluster.Namespace,
	}
	err = cw.DeleteByKeyIgnoreNotFound(cheCtx.Context, gatewayConfigKey, &corev1.ConfigMap{})
	if err != nil {
		logger.Error(err, "failed to delete gateway ConfigMap", "Name", gatewayConfigKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(cheCtx.Context, objKey, &corev1.ConfigMap{})
	if err != nil {
		logger.Error(err, "Failed to delete ConfigMap", "Name", objKey.Name)
	}

	err = cw.DeleteByKeyIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{
			Name:      constants.OpenVSXServerExtensionsConfigMapName,
			Namespace: cheCtx.CheCluster.Namespace,
		},
		&corev1.ConfigMap{},
	)
	if err != nil {
		logger.Error(err, "Failed to delete ConfigMap", "Name", constants.OpenVSXServerExtensionsConfigMapName)
	}

	err = cw.DeleteByKeyIgnoreNotFound(cheCtx.Context, objKey, &corev1.PersistentVolumeClaim{})
	if err != nil {
		logger.Error(err, "Failed to delete PVC", "Name", objKey.Name)
	}

	if cheCtx.CheCluster.Status.OpenVSXURL != "" {
		cheCtx.CheCluster.Status.OpenVSXURL = ""

		if err = deploy.UpdateCheCRStatus(cheCtx, "status: OpenVSXURL", ""); err != nil {
			logger.Error(err, "Failed to update status for OpenVSXURL")
		}
	}
}

func (r *OpenVSXServerReconciler) isServerReady(cheCtx *chetypes.CheContext) bool {
	actual := &appsv1.Deployment{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXServerComponentName, Namespace: cheCtx.CheCluster.Namespace},
		actual,
	)
	if !exists || err != nil {
		return false
	}
	return actual.Status.AvailableReplicas > 0 && actual.Status.UnavailableReplicas == 0
}

func (r *OpenVSXServerReconciler) Finalize(_ *chetypes.CheContext) bool {
	return true
}
