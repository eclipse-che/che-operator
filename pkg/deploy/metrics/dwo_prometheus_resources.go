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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	dwoServiceMonitorName        = "devworkspace"
	dwoPrometheusRoleName        = "devworkspace-prometheus"
	dwoPrometheusRoleBindingName = "devworkspace-prometheus"
)

type DWOPrometheusResourceProvider struct{}

func (r *DWOPrometheusResourceProvider) GetPrometheusRoleBinding(ctx *chetypes.DeployContext) (*rbacv1.RoleBinding, error) {
	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	roleBinding := &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      dwoPrometheusRoleBindingName,
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
			Name:     dwoPrometheusRoleName,
		},
	}

	if namespace == ctx.CheCluster.Namespace {
		if err := controllerutil.SetControllerReference(ctx.CheCluster, roleBinding, ctx.ClusterAPI.Scheme); err != nil {
			return nil, err
		}
	}

	return roleBinding, nil
}

func (r *DWOPrometheusResourceProvider) GetPrometheusRole(ctx *chetypes.DeployContext) (*rbacv1.Role, error) {
	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	role := &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dwoPrometheusRoleName,
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
	}

	if namespace == ctx.CheCluster.Namespace {
		if err := controllerutil.SetControllerReference(ctx.CheCluster, role, ctx.ClusterAPI.Scheme); err != nil {
			return nil, err
		}
	}

	return role, nil
}

func (r *DWOPrometheusResourceProvider) GetServiceMonitor(ctx *chetypes.DeployContext) (*monitoringv1.ServiceMonitor, error) {
	operatorNamespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	interval, err := getServiceMonitorInterval(ctx, dwoServiceMonitorName, ctx.CheCluster.Namespace)
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
			Name:      dwoServiceMonitorName,
			Labels:    deploy.GetLabels(constants.MetricsComponentName),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Endpoints: []monitoringv1.Endpoint{
				{
					Interval:        interval,
					Scheme:          "https",
					Port:            metricsPortName,
					BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
					TLSConfig: &monitoringv1.TLSConfig{
						SafeTLSConfig: monitoringv1.SafeTLSConfig{
							InsecureSkipVerify: ptr.To(true),
						},
					},
				},
			},
			NamespaceSelector: monitoringv1.NamespaceSelector{
				MatchNames: []string{
					operatorNamespace,
				},
			},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					constants.KubernetesNameLabelKey: constants.DevWorkspaceControllerName,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, serviceMonitor, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return serviceMonitor, nil
}
