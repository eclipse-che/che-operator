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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	cheServerServiceMonitorNameTemplate        = "%s"
	cheServerPrometheusRoleNameTemplate        = "%s-prometheus"
	cheServerPrometheusRoleBindingNameTemplate = "%s-prometheus"
)

type CheServerPrometheusResourceProvider struct{}

func (r *CheServerPrometheusResourceProvider) GetPrometheusRoleBinding(ctx *chetypes.DeployContext) (*rbacv1.RoleBinding, error) {
	roleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, defaults.GetCheFlavor())
	roleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, defaults.GetCheFlavor())

	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.CheCluster.Namespace,
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
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, roleBinding, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return roleBinding, nil
}

func (r *CheServerPrometheusResourceProvider) GetPrometheusRole(ctx *chetypes.DeployContext) (*rbacv1.Role, error) {
	roleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, defaults.GetCheFlavor())

	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.MetricsComponentName),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, role, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return role, nil
}

func (r *CheServerPrometheusResourceProvider) GetServiceMonitor(ctx *chetypes.DeployContext) (*monitoringv1.ServiceMonitor, error) {
	serviceMonitorName := fmt.Sprintf(cheServerServiceMonitorNameTemplate, defaults.GetCheFlavor())

	interval, err := getServiceMonitorInterval(ctx, serviceMonitorName, ctx.CheCluster.Namespace)
	if err != nil {
		return nil, err
	}

	serviceMonitor := &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ctx.CheCluster.Namespace,
			Name:      serviceMonitorName,
			Labels:    deploy.GetLabels(constants.MetricsComponentName),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval: interval,
					Scheme:   "http",
					Port:     metricsPortName,
				},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{
					ctx.CheCluster.Namespace,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.KubernetesNameLabelKey: defaults.GetCheFlavor(),
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, serviceMonitor, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return serviceMonitor, nil
}
