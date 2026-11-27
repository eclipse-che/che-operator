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

package dashboard

import (
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy/gateway"
)

const (
	exposePath = "/dashboard/"
)

var (
	logger = logf.Log.WithName("dashboard")
)

type DashboardReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *DashboardReconciler {
	return &DashboardReconciler{}
}

func (r *DashboardReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := r.syncAll(cheCtx); err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to reconcile Dashboard: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *DashboardReconciler) syncAll(cheCtx *chetypes.CheContext) error {
	if err := r.syncService(cheCtx); err != nil {
		return err
	}

	if err := r.syncGatewayConfig(cheCtx); err != nil {
		return err
	}

	if err := r.syncServiceAccount(cheCtx); err != nil {
		return err
	}

	if err := r.syncClusterRole(cheCtx); err != nil {
		return err
	}

	if err := r.syncClusterRoleBinding(cheCtx); err != nil {
		return err
	}

	if err := r.syncDeployment(cheCtx); err != nil {
		return err
	}

	if err := r.deleteLegacyRBAC(cheCtx); err != nil {
		return err
	}

	return nil
}

func (r *DashboardReconciler) syncService(cheCtx *chetypes.CheContext) error {
	labels := chelabels.GetStandardLabels(getComponentName())

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(),
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt32(8080),
					Protocol:   "TCP",
				},
			},
			Selector: labels,
		},
	}

	return resources.Service.Sync(cheCtx, service)
}

func (r *DashboardReconciler) syncGatewayConfig(cheCtx *chetypes.CheContext) error {
	cfg := r.createGatewayConfig(cheCtx)

	cm, err := gateway.GetConfigmapForGatewayConfig(cheCtx, getComponentName(), cfg)
	if err != nil {
		return err
	}

	return resources.ConfigMap.Sync(cheCtx, cm)
}

func (r *DashboardReconciler) createGatewayConfig(cheCtx *chetypes.CheContext) *gateway.TraefikConfig {
	componentName := getComponentName()
	rule := fmt.Sprintf("Path(`/`) || Path(`/f`) || PathPrefix(`%s`)", exposePath)
	serviceAddr := fmt.Sprintf("http://%s:8080", componentName)

	cfg := gateway.CreateCommonTraefikConfig(componentName, rule, 10, serviceAddr, []string{})
	if cheCtx.CheCluster.IsAccessTokenConfigured() {
		cfg.AddAuthHeaderRewrite(componentName)
	}

	return cfg
}

func (r *DashboardReconciler) syncServiceAccount(cheCtx *chetypes.CheContext) error {
	labels := chelabels.GetStandardLabels(getComponentName())

	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(),
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    labels,
		},
	}

	return client.IgnoreAlreadyExists(resources.ServiceAccount.Create(cheCtx, sa))
}

func (r *DashboardReconciler) syncClusterRole(cheCtx *chetypes.CheContext) error {
	labels := chelabels.GetStandardLabels(getComponentName())

	clusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   getComponentName(),
			Labels: labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"workspace.devfile.io"},
				Resources: []string{"devworkspaces"},
				Verbs:     []string{"create", "update", "patch", "get", "watch", "list", "delete"},
			},
			{
				APIGroups: []string{"workspace.devfile.io"},
				Resources: []string{"devworkspacetemplates"},
				Verbs:     []string{"create", "get", "list", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "create", "update", "list"},
			},
			{
				APIGroups: []string{"org.eclipse.che"},
				Resources: []string{"checlusters"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"get", "list"},
			},
			{
				APIGroups: []string{"controller.devfile.io"},
				Resources: []string{"devworkspaceoperatorconfigs"},
				Verbs:     []string{"get"},
			},
		},
	}

	return resources.ClusterRole.Sync(cheCtx, clusterRole)
}

func (r *DashboardReconciler) syncClusterRoleBinding(cheCtx *chetypes.CheContext) error {
	labels := chelabels.GetStandardLabels(getComponentName())

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   getComponentName(),
			Labels: labels,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      getComponentName(),
				Namespace: cheCtx.CheCluster.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     getComponentName(),
			Kind:     "ClusterRole",
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	return resources.ClusterRoleBinding.Sync(cheCtx, clusterRoleBinding)
}

func (r *DashboardReconciler) syncDeployment(cheCtx *chetypes.CheContext) error {
	deployment, err := r.getDashboardDeploymentSpec(cheCtx)
	if err != nil {
		return err
	}

	return resources.Deployment.Sync(cheCtx, deployment)
}

func (r *DashboardReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	return r.deleteAll(cheCtx)
}

func (r *DashboardReconciler) deleteAll(cheCtx *chetypes.CheContext) error {
	if err := resources.ClusterRole.Delete(cheCtx, getComponentName()); err != nil {
		return err
	}

	if err := resources.ClusterRoleBinding.Delete(cheCtx, getComponentName()); err != nil {
		return err
	}

	if err := r.deleteLegacyRBAC(cheCtx); err != nil {
		return err
	}

	return nil
}

func (r *DashboardReconciler) deleteLegacyRBAC(cheCtx *chetypes.CheContext) error {
	clusterRoleBindingName := fmt.Sprintf("%s-che-dashboard", cheCtx.CheCluster.Namespace)
	if err := resources.ClusterRoleBinding.Delete(cheCtx, clusterRoleBindingName); err != nil {
		return err
	}

	clusterRoleName := fmt.Sprintf("%s-che-dashboard", cheCtx.CheCluster.Namespace)
	if err := resources.ClusterRole.Delete(cheCtx, clusterRoleName); err != nil {
		return err
	}

	return nil
}

func getComponentName() string {
	return defaults.GetCheFlavor() + "-dashboard"
}
