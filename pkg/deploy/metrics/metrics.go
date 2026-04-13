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

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/pointer"
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
	openshiftMonitoringLabel = "openshift.io/cluster-monitoring"
)

var log = ctrl.Log.WithName("metrics")

type PrometheusResourceProvider interface {
	GetPrometheusRole(*chetypes.DeployContext) (*rbacv1.Role, error)
	GetPrometheusRoleBinding(*chetypes.DeployContext) (*rbacv1.RoleBinding, error)
	GetServiceMonitor(*chetypes.DeployContext) (*monitoringv1.ServiceMonitor, error)
}

type PrometheusResource struct {
	object   client.Object
	diffOpts []cmp.Option
}

type MetricsReconciler struct {
	reconciler.Reconcilable
}

func NewMetricsReconciler() *MetricsReconciler {
	return &MetricsReconciler{}
}

func (r *MetricsReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	isDWOMetricsEnabled := pointer.BoolDeref(ctx.CheCluster.Spec.DevEnvironments.Metrics, constants.DefaultDWOMetricsEnabled)
	if isDWOMetricsEnabled {
		if err := syncResources(ctx, &DWOPrometheusResourceProvider{}); err != nil {
			return reconcile.Result{}, false, err
		}
	} else {
		if err := deleteResources(ctx, &DWOPrometheusResourceProvider{}); err != nil {
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

	if err := addOpenShiftMonitoringLabel(ctx); err != nil {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (r *MetricsReconciler) Finalize(ctx *chetypes.DeployContext) bool {
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
				Name:      resource.object.GetName(),
				Namespace: resource.object.GetNamespace(),
			},
			resource.object,
		); err != nil {
			log.Error(err, "Failed to delete resource", "name", resource.object.GetName(), "namespace", resource.object.GetNamespace())
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
			resource.object,
			&k8sclient.SyncOptions{
				DiffOpts: resource.diffOpts,
			},
		); err != nil {
			return err
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
				Name:      resource.object.GetName(),
				Namespace: resource.object.GetNamespace(),
			},
			resource.object,
		); err != nil {
			return err
		}
	}

	return nil
}

func collectPrometheusResources(ctx *chetypes.DeployContext, prometheusResourceProvider PrometheusResourceProvider) ([]PrometheusResource, error) {
	var prometheusResources []PrometheusResource

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

	prometheusResources = append(prometheusResources, PrometheusResource{object: role, diffOpts: diffs.Role})
	prometheusResources = append(prometheusResources, PrometheusResource{object: roleBinding, diffOpts: diffs.RoleBinding})
	prometheusResources = append(prometheusResources, PrometheusResource{object: serviceMonitor, diffOpts: getServiceMonitorWithIgnoredIntervalDiffs()})

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

func getServiceMonitorWithIgnoredIntervalDiffs() cmp.Options {
	return cmp.Options{
		diffs.ServiceMonitor,
		cmpopts.IgnoreFields(monitoringv1.Endpoint{}, "Interval"),
	}
}
