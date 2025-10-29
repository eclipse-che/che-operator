//
// Copyright (c) 2019-2025 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package containercapabilities

import (
	"context"
	"fmt"
	"time"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/types"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	logger = ctrl.Log.WithName("containercapabilities")
)

type ContainerCapability interface {
	GetUserRoleName() string
	GetUserClusterRoleBindingName() string
	getDWOClusterRoleName() string
	getDWOClusterRoleBindingName() string
	getSCCName(cheCluster *chev2.CheCluster) string
	getDefaultSCCName() string
	getSCCSpec(sccName string) *securityv1.SecurityContextConstraints
	getFinalizer() string
}

type ContainerCapabilitiesReconciler struct {
	reconciler.Reconcilable

	containerRunCapability   *ContainerRun
	containerBuildCapability *ContainerBuild
}

func NewContainerCapabilitiesReconciler() *ContainerCapabilitiesReconciler {
	return &ContainerCapabilitiesReconciler{
		containerRunCapability:   NewContainerRun(),
		containerBuildCapability: NewContainerBuild(),
	}
}

func (r *ContainerCapabilitiesReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	// If container build/run capabilities are enabled, then container build/run configuration is supposed to be set as well
	// with default values, see `api/v2/checluster_webhook.go` and `api/v2/checluster_types.go` (for default values).
	if ctx.CheCluster.IsContainerBuildCapabilitiesEnabled() {
		if err := r.sync(ctx, r.containerBuildCapability); err != nil {
			return reconcile.Result{RequeueAfter: 1 * time.Second}, false, err
		}
	} else {
		if err := r.delete(ctx, r.containerBuildCapability); err != nil {
			return reconcile.Result{RequeueAfter: 1 * time.Second}, false, err
		}
	}

	if ctx.CheCluster.IsContainerRunCapabilitiesEnabled() {
		if err := r.sync(ctx, r.containerRunCapability); err != nil {
			return reconcile.Result{RequeueAfter: 1 * time.Second}, false, err
		}
	} else {
		if err := r.delete(ctx, r.containerRunCapability); err != nil {
			return reconcile.Result{RequeueAfter: 1 * time.Second}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (r *ContainerCapabilitiesReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	if err := r.delete(ctx, r.containerBuildCapability); err != nil {
		logger.Error(err, "Failed to delete container build capability resources")
		return false
	}

	if err := r.delete(ctx, r.containerRunCapability); err != nil {
		logger.Error(err, "Failed to delete container run capability resources")
		return false
	}

	return true
}

func (r *ContainerCapabilitiesReconciler) sync(ctx *chetypes.DeployContext, cc ContainerCapability) error {
	sccName := cc.getSCCName(ctx.CheCluster)
	if sccName == "" {
		return nil
	}

	devWorkspaceServiceAccountNamespace, err := r.getDevWorkspaceServiceAccountNamespace(ctx)
	if err != nil {
		return err
	}

	if err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		r.getDWOClusterRole(
			sccName,
			cc.getDWOClusterRoleName(),
		),
		&k8sclient.SyncOptions{DiffOpts: diffs.ClusterRole},
	); err != nil {
		return err
	}

	if err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		r.getDWClusterRoleBinding(
			devWorkspaceServiceAccountNamespace,
			cc.getDWOClusterRoleName(),
			cc.getDWOClusterRoleBindingName(),
		),
		&k8sclient.SyncOptions{DiffOpts: diffs.ClusterRoleBinding},
	); err != nil {
		return err
	}

	if err := ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		r.getUserClusterRole(
			sccName,
			cc.GetUserRoleName(),
		),
		&k8sclient.SyncOptions{DiffOpts: diffs.ClusterRole},
	); err != nil {
		return err
	}

	sccKey := types.NamespacedName{Name: sccName}

	scc := &securityv1.SecurityContextConstraints{}
	if exists, err := ctx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(context.TODO(), sccKey, scc); exists {
		if deploy.IsPartOfEclipseCheResourceAndManagedByOperator(scc.Labels) {
			// SCC exists and created by operator (custom SCC won't be updated).
			// So, remove priority. See details https://issues.redhat.com/browse/CRW-3894
			scc.Priority = nil

			if err := ctx.ClusterAPI.NonCachingClientWrapper.Sync(
				context.TODO(),
				scc,
			); err != nil {
				return err
			}
		}
	} else if err == nil {
		if err := ctx.ClusterAPI.NonCachingClientWrapper.Create(
			context.TODO(),
			cc.getSCCSpec(sccName),
		); err != nil {
			return err
		}
	} else {
		return err
	}

	if err := deploy.AppendFinalizer(ctx, cc.getFinalizer()); err != nil {
		return err
	}

	return nil
}

func (r *ContainerCapabilitiesReconciler) delete(ctx *chetypes.DeployContext, cc ContainerCapability) error {
	if err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: cc.getDWOClusterRoleBindingName()},
		&rbacv1.ClusterRoleBinding{},
	); err != nil {
		return err
	}

	if err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: cc.getDWOClusterRoleName()},
		&rbacv1.ClusterRole{},
	); err != nil {
		return err
	}

	if err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: cc.GetUserRoleName()},
		&rbacv1.ClusterRole{},
	); err != nil {
		return err
	}

	sccName := utils.GetValue(cc.getSCCName(ctx.CheCluster), cc.getDefaultSCCName())
	sccKey := types.NamespacedName{Name: sccName}

	scc := &securityv1.SecurityContextConstraints{}
	if exists, err := ctx.ClusterAPI.NonCachingClientWrapper.GetIgnoreNotFound(context.TODO(), sccKey, scc); exists {
		// Removes only if it is managed by operator
		if scc.Labels[constants.KubernetesManagedByLabelKey] == deploy.GetManagedByLabel() {
			if err = ctx.ClusterAPI.NonCachingClientWrapper.DeleteByKeyIgnoreNotFound(
				context.TODO(),
				sccKey,
				&securityv1.SecurityContextConstraints{}); err != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	}

	if err := deploy.DeleteFinalizer(ctx, cc.getFinalizer()); err != nil {
		return err
	}

	return nil
}

// getDevWorkspaceServiceAccountNamespace returns the namespace of the DevWorkspace ServiceAccount.
// It searches for the DevWorkspace Operator Pods by its labels.
func (r *ContainerCapabilitiesReconciler) getDevWorkspaceServiceAccountNamespace(ctx *chetypes.DeployContext) (string, error) {
	selector := labels.SelectorFromSet(
		labels.Set{
			constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
			constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
		},
	)

	items, err := ctx.ClusterAPI.NonCachingClientWrapper.List(
		context.TODO(),
		&corev1.PodList{},
		&client.ListOptions{LabelSelector: selector},
	)
	if err != nil {
		return "", err
	}

	for _, item := range items {
		pod := item.(*corev1.Pod)
		if pod.Spec.ServiceAccountName == constants.DevWorkspaceServiceAccountName {
			return pod.Namespace, nil
		}
	}

	return "", fmt.Errorf("ServiceAccount %s not found", constants.DevWorkspaceServiceAccountName)
}

func (r *ContainerCapabilitiesReconciler) getUserClusterRole(sccName string, clusterRoleName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				Verbs:         []string{"use"},
				ResourceNames: []string{sccName},
			},
		},
	}
}

func (r *ContainerCapabilitiesReconciler) getDWOClusterRole(sccName string, clusterRoleName string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleName,
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				Verbs:         []string{"get", "update", "use"},
				ResourceNames: []string{sccName},
			},
		},
	}
}

func (r *ContainerCapabilitiesReconciler) getDWClusterRoleBinding(
	devWorkspaceNamespace string,
	clusterRoleName string,
	clusterRoleBindingName string,
) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   clusterRoleBindingName,
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      constants.DevWorkspaceServiceAccountName,
				Namespace: devWorkspaceNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
		},
	}
}
