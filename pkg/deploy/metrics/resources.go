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
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

const (
	prometheusServiceAccount     = "prometheus-k8s"
	openShiftMonitoringNamespace = "openshift-monitoring"

	metricsPortName = "metrics"

	DefaultMetricsUpdateInterval monitoringv1.Duration = "10s"
)

func getPrometheusRoleBinding(roleBindingName, roleName, namespace string) (*rbacv1.RoleBinding, error) {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      roleBindingName,
			Labels:    deploy.GetLabels(constants.MetricsComponentName),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      getPrometheusServiceAccount(),
				Namespace: getOpenShiftMonitoringNamespace(),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
	}, nil
}

func getPrometheusRole(roleName, namespace string) (*rbacv1.Role, error) {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
			Labels:    deploy.GetLabels(constants.MetricsComponentName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}, nil
}

func getServiceMonitor(
	ctx *chetypes.DeployContext,
	name, namespace, serviceNamespace string,
	matchLabels map[string]string,
) (*monitoringv1.ServiceMonitor, error) {

	serviceMonitor := &monitoringv1.ServiceMonitor{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: name, Namespace: namespace},
		serviceMonitor,
	)
	if err != nil {
		return nil, err
	}

	interval := DefaultMetricsUpdateInterval
	if exists && len(serviceMonitor.Spec.Endpoints) == 1 {
		interval = serviceMonitor.Spec.Endpoints[0].Interval
	}

	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    deploy.GetLabels(constants.MetricsComponentName),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: interval,
					Scheme:   ptr.To(monitoringv1.SchemeHTTP),
					Port:     metricsPortName,
				},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{
					serviceNamespace,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
		},
	}, nil
}

func getOpenShiftMonitoringNamespace() string {
	return utils.GetValue(os.Getenv("CHE_OPERATOR__OPENSHIFT_MONITORING_NAMESPACE"), openShiftMonitoringNamespace)
}

func getPrometheusServiceAccount() string {
	return utils.GetValue(os.Getenv("CHE_OPERATOR__PROMETHEUS_SERVICE_ACCOUNT"), prometheusServiceAccount)
}
