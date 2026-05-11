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
	"os"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	prometheusServiceAccount     = "prometheus-k8s"
	openShiftMonitoringNamespace = "openshift-monitoring"

	defaultMetricsUpdateInterval monitoringv1.Duration = "10s"
)

func getServiceMonitorInterval(ctx *chetypes.DeployContext, name, namespace string) (monitoringv1.Duration, error) {
	serviceMonitor := &monitoringv1.ServiceMonitor{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		serviceMonitor,
	)
	if err != nil {
		return "", err
	}

	if exists && len(serviceMonitor.Spec.Endpoints) == 1 && serviceMonitor.Spec.Endpoints[0].Interval != "" {
		return serviceMonitor.Spec.Endpoints[0].Interval, nil
	}

	return defaultMetricsUpdateInterval, nil
}

func getOpenShiftMonitoringNamespace() string {
	return utils.GetValue(os.Getenv("CHE_OPERATOR__OPENSHIFT_MONITORING_NAMESPACE"), openShiftMonitoringNamespace)
}

func getPrometheusServiceAccount() string {
	return utils.GetValue(os.Getenv("CHE_OPERATOR__PROMETHEUS_SERVICE_ACCOUNT"), prometheusServiceAccount)
}

func getServiceMonitorWithIgnoredIntervalDiffs() cmp.Options {
	return cmp.Options{
		diffs.ServiceMonitor,
		cmpopts.IgnoreFields(monitoringv1.Endpoint{}, "Interval"),
	}
}
