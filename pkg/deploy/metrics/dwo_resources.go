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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// Accordingly to the documentation ServiceMonitor name
	DWOServiceMonitorName = "devworkspace-controller"

	dwoPrometheusRoleName        = "devworkspace-prometheus"
	dwoPrometheusRoleBindingName = "devworkspace-prometheus"
)

type DWOPrometheusResourceProvider struct{}

func (r *DWOPrometheusResourceProvider) GetPrometheusRoleBinding(ctx *chetypes.DeployContext) (*rbacv1.RoleBinding, error) {
	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	roleBinding, err := getPrometheusRoleBinding(dwoPrometheusRoleBindingName, dwoPrometheusRoleName, namespace)
	if err != nil {
		return nil, err
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

	role, err := getPrometheusRole(dwoPrometheusRoleName, namespace)
	if err != nil {
		return nil, err
	}

	if namespace == ctx.CheCluster.Namespace {
		if err := controllerutil.SetControllerReference(ctx.CheCluster, role, ctx.ClusterAPI.Scheme); err != nil {
			return nil, err
		}
	}

	return role, nil
}

func (r *DWOPrometheusResourceProvider) GetServiceMonitor(ctx *chetypes.DeployContext) (*monitoringv1.ServiceMonitor, error) {
	namespace, err := infrastructure.GetOperatorNamespace()
	if err != nil {
		return nil, err
	}

	serviceMonitor, err := getServiceMonitor(ctx,
		DWOServiceMonitorName,
		ctx.CheCluster.Namespace,
		namespace,
		map[string]string{
			constants.KubernetesNameLabelKey: constants.DevWorkspaceControllerName,
		},
	)
	if err != nil {
		return nil, err
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, serviceMonitor, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return serviceMonitor, nil
}
