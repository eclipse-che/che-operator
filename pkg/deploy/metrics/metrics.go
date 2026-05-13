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

package metrics

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	metricsPortName = "metrics"

	openshiftMonitoringLabel = "openshift.io/cluster-monitoring"
)

var (
	log                         = ctrl.Log.WithName("metrics")
	isAbandonedResourcesDeleted = false
)

type PrometheusResourceProvider interface {
	GetPrometheusRole(*chetypes.DeployContext) (*rbacv1.Role, error)
	GetPrometheusRoleBinding(*chetypes.DeployContext) (*rbacv1.RoleBinding, error)
	GetServiceMonitor(*chetypes.DeployContext) (*monitoringv1.ServiceMonitor, error)
}

type MetricsReconciler struct {
	reconciler.Reconcilable
}

func NewMetricsReconciler() *MetricsReconciler {
	return &MetricsReconciler{}
}

func (r *MetricsReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if err := syncResources(ctx, &DWOPrometheusResourceProvider{}); err != nil {
		return reconcile.Result{}, false, err
	}

	if infrastructure.IsOpenShift() {
		if err := addOpenShiftMonitoringLabel(ctx); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	isCheServerMetricsEnabled := ctx.CheCluster.Spec.Components.Metrics.Enable
	if isCheServerMetricsEnabled {
		if err := syncResources(ctx, &CheServerPrometheusResourceProvider{}); err != nil {
			return reconcile.Result{}, false, err
		}
	} else {
		if err := deleteResources(ctx, &CheServerPrometheusResourceProvider{}); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	if !isAbandonedResourcesDeleted {
		if err := deleteAbandonedResources(ctx); err != nil {
			return reconcile.Result{}, false, err
		}

		// We don't need to delete them on every reconcile loop
		isAbandonedResourcesDeleted = true
	}

	return reconcile.Result{}, true, nil
}

func (r *MetricsReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	// Do not remove the openshift.io/cluster-monitoring label,
	// as it may have already existed.

	cheServerPrometheusResources, err := collectPrometheusResources(ctx, &CheServerPrometheusResourceProvider{})
	if err != nil {
		log.Error(err, "Failed to collect Prometheus resources")
		return false
	}

	dwoPrometheusResources, err := collectPrometheusResources(ctx, &DWOPrometheusResourceProvider{})
	if err != nil {
		log.Error(err, "Failed to collect Prometheus resources")
		return false
	}

	prometheusResources := append(cheServerPrometheusResources, dwoPrometheusResources...)

	done := true
	for _, resource := range prometheusResources {
		if err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{
				Name:      resource.Object.GetName(),
				Namespace: resource.Object.GetNamespace(),
			},
			resource.Object,
		); err != nil {
			log.Error(err, "Failed to delete resource", "name", resource.Object.GetName(), "namespace", resource.Object.GetNamespace())
			done = false
		}
	}

	return done
}

func syncResources(ctx *chetypes.DeployContext, prometheusResourceProvider PrometheusResourceProvider) error {
	prometheusResources, err := collectPrometheusResources(ctx, prometheusResourceProvider)
	if err != nil {
		return err
	}

	for _, resource := range prometheusResources {
		if err := ctx.ClusterAPI.ClientWrapper.Sync(
			context.TODO(),
			resource.Object,
			&k8sclient.SyncOptions{
				DiffOpts: resource.DiffOpts,
			},
		); err != nil {
			return errors.Wrap(err, "Failed to sync resource")
		}
	}

	return nil
}

func deleteResources(ctx *chetypes.DeployContext, prometheusResourceProvider PrometheusResourceProvider) error {
	prometheusResources, err := collectPrometheusResources(ctx, prometheusResourceProvider)
	if err != nil {
		return err
	}

	for _, resource := range prometheusResources {
		if err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
			context.TODO(),
			types.NamespacedName{
				Name:      resource.Object.GetName(),
				Namespace: resource.Object.GetNamespace(),
			},
			resource.Object,
		); err != nil {
			return errors.Wrap(err, "Failed to delete resource")
		}
	}

	return nil
}

func collectPrometheusResources(ctx *chetypes.DeployContext, prometheusResourceProvider PrometheusResourceProvider) ([]k8sclient.SyncTarget, error) {
	var prometheusResources []k8sclient.SyncTarget

	role, err := prometheusResourceProvider.GetPrometheusRole(ctx)
	if err != nil {
		return prometheusResources, err
	}
	roleBinding, err := prometheusResourceProvider.GetPrometheusRoleBinding(ctx)
	if err != nil {
		return prometheusResources, err
	}
	serviceMonitor, err := prometheusResourceProvider.GetServiceMonitor(ctx)
	if err != nil {
		return prometheusResources, err
	}

	prometheusResources = append(prometheusResources, k8sclient.SyncTarget{Object: role, DiffOpts: diffs.Role})
	prometheusResources = append(prometheusResources, k8sclient.SyncTarget{Object: roleBinding, DiffOpts: diffs.RoleBinding})
	prometheusResources = append(prometheusResources, k8sclient.SyncTarget{Object: serviceMonitor, DiffOpts: getServiceMonitorWithIgnoredIntervalDiffs()})

	return prometheusResources, nil
}

func addOpenShiftMonitoringLabel(ctx *chetypes.DeployContext) error {
	namespace := &corev1.Namespace{}
	if err := ctx.ClusterAPI.NonCachingClient.Get(
		context.TODO(),
		types.NamespacedName{Name: ctx.CheCluster.Namespace},
		namespace,
	); err != nil {
		return err
	}

	if namespace.Labels[openshiftMonitoringLabel] == "true" {
		return nil
	}

	patch := []byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":"true"}}}`, openshiftMonitoringLabel))
	if err := ctx.ClusterAPI.NonCachingClient.Patch(
		context.TODO(),
		namespace,
		client.RawPatch(types.MergePatchType, patch),
	); err != nil {
		return err
	}

	return nil
}

// Deletes abandoned resources, that previously were mentioned in the documentation.
func deleteAbandonedResources(ctx *chetypes.DeployContext) error {
	operatorNamespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return err
	}

	syncObjects := []k8sclient.SyncTarget{
		{
			Object: &monitoringv1.ServiceMonitor{},
			Key:    types.NamespacedName{Name: "che-host", Namespace: ctx.CheCluster.Namespace},
		},
		{
			Object: &monitoringv1.ServiceMonitor{},
			Key:    types.NamespacedName{Name: "devworkspace-controller", Namespace: ctx.CheCluster.Namespace},
		},
		{
			Object: &monitoringv1.ServiceMonitor{},
			Key:    types.NamespacedName{Name: "openshift-devspaces-metrics-exporter", Namespace: operatorNamespace},
		},
		{
			Object: &rbacv1.Role{},
			Key:    types.NamespacedName{Name: "prometheus-k8s", Namespace: ctx.CheCluster.Namespace},
		},
		{
			Object: &rbacv1.Role{},
			Key:    types.NamespacedName{Name: "prometheus-k8s", Namespace: operatorNamespace},
		},
		{
			Object: &rbacv1.RoleBinding{},
			Key:    types.NamespacedName{Name: fmt.Sprintf("view-%s-openshift-monitoring-prometheus-k8s", defaults.GetCheFlavor()), Namespace: ctx.CheCluster.Namespace},
		},
		{
			Object: &rbacv1.RoleBinding{},
			Key:    types.NamespacedName{Name: fmt.Sprintf("view-%s-openshift-monitoring-prometheus-k8s", defaults.GetCheFlavor()), Namespace: operatorNamespace},
		},
		{
			Object: &rbacv1.RoleBinding{},
			Key:    types.NamespacedName{Name: "view-openshift-monitoring-prometheus-k8s", Namespace: operatorNamespace},
		},
	}

	for _, syncObject := range syncObjects {
		err := ctx.ClusterAPI.NonCachingClientWrapper.DeleteByKeyIgnoreNotFound(
			context.TODO(),
			syncObject.Key,
			syncObject.Object,
		)
		if err != nil {
			return errors.Wrap(err, "Failed to delete resource")
		}
	}

	return nil
}
