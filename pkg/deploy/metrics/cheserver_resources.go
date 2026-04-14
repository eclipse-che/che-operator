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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// Accordingly to the documentation ServiceMonitor name
	CheServerServiceMonitorName = "che-host"

	cheServerPrometheusRoleNameTemplate        = "%s-prometheus"
	cheServerPrometheusRoleBindingNameTemplate = "%s-prometheus"
)

type CheServerPrometheusResourceProvider struct{}

func (r *CheServerPrometheusResourceProvider) GetPrometheusRoleBinding(ctx *chetypes.DeployContext) (*rbacv1.RoleBinding, error) {
	roleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, defaults.GetCheFlavor())
	roleBindingName := fmt.Sprintf(cheServerPrometheusRoleBindingNameTemplate, defaults.GetCheFlavor())

	roleBinding, err := getPrometheusRoleBinding(roleBindingName, roleName, ctx.CheCluster.Namespace)
	if err != nil {
		return nil, err
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, roleBinding, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return roleBinding, nil
}

func (r *CheServerPrometheusResourceProvider) GetPrometheusRole(ctx *chetypes.DeployContext) (*rbacv1.Role, error) {
	roleName := fmt.Sprintf(cheServerPrometheusRoleNameTemplate, defaults.GetCheFlavor())

	role, err := getPrometheusRole(roleName, ctx.CheCluster.Namespace)
	if err != nil {
		return nil, err
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, role, ctx.ClusterAPI.Scheme); err != nil {
		return nil, err
	}

	return role, nil
}

func (r *CheServerPrometheusResourceProvider) GetServiceMonitor(ctx *chetypes.DeployContext) (*monitoringv1.ServiceMonitor, error) {
	serviceMonitor, err := getServiceMonitor(
		ctx,
		CheServerServiceMonitorName,
		ctx.CheCluster.Namespace,
		ctx.CheCluster.Namespace,
		map[string]string{
			constants.KubernetesNameLabelKey: defaults.GetCheFlavor(),
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
