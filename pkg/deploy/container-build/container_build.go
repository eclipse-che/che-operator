//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package containerbuild

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/types"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ContainerBuildReconciler struct {
	deploy.Reconcilable
}

func NewContainerBuildReconciler() *ContainerBuildReconciler {
	return &ContainerBuildReconciler{}
}

func (cb *ContainerBuildReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if ctx.CheCluster.IsContainerBuildCapabilitiesEnabled() {
		// If container build capabilities are enabled, then container build configuration is supposed to be set
		// with default values, see `api/v2/checluster_webhook.go` and `api/v2/checluster_types.go` (for default values).
		// The check below to avoid NPE while CheCluster is not updated with defaults.
		if ctx.CheCluster.IsOpenShiftSecurityContextConstraintSet() {
			if done, err := cb.syncSCC(ctx); !done {
				return reconcile.Result{}, false, err
			}

			if done, err := cb.syncRBAC(ctx); !done {
				return reconcile.Result{}, false, err
			}

			if err := deploy.AppendFinalizer(ctx, cb.getFinalizerName()); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	} else {
		if done, err := cb.removeRBAC(ctx); !done {
			return reconcile.Result{}, false, err
		}

		if done, err := cb.removeSCC(ctx); !done {
			return reconcile.Result{}, false, err
		}

		if err := deploy.DeleteFinalizer(ctx, cb.getFinalizerName()); err != nil {
			return reconcile.Result{}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}

func (cb *ContainerBuildReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	done := true

	if done, err := cb.removeRBAC(ctx); !done {
		done = false
		logrus.Errorf("Failed to delete RBAC, cause: %v", err)
	}

	if done, err := cb.removeSCC(ctx); !done {
		done = false
		logrus.Errorf("Failed to delete SCC, cause: %v", err)
	}

	if err := deploy.DeleteFinalizer(ctx, cb.getFinalizerName()); err != nil {
		done = false
		logrus.Errorf("Failed to delete finalizer, cause: %v", err)
	}

	return done
}

func (cb *ContainerBuildReconciler) syncSCC(ctx *chetypes.DeployContext) (bool, error) {
	scc := &securityv1.SecurityContextConstraints{}
	if exists, err := deploy.Get(ctx,
		types.NamespacedName{Name: ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint},
		scc); err != nil {
		return false, err
	} else if exists {
		if deploy.IsPartOfEclipseCheResourceAndManagedByOperator(scc.Labels) {
			// SCC exists and created by operator (custom SCC won't be updated).
			// So, remove priority. See details https://issues.redhat.com/browse/CRW-3894
			scc.Priority = nil

			// Ensure kind and version set correctly before invoking `Sync`
			scc.Kind = "SecurityContextConstraints"
			scc.APIVersion = securityv1.GroupVersion.String()

			return deploy.Sync(
				ctx,
				scc,
				cmp.Options{
					cmp.Comparer(func(x, y securityv1.SecurityContextConstraints) bool {
						return pointer.Int32Equal(x.Priority, y.Priority)
					}),
				})
		}
	} else {
		// Create a new SCC. If custom SCC exists then it won't be touched.
		return deploy.Create(ctx, cb.getSccSpec(ctx))
	}

	return true, nil
}

func (cb *ContainerBuildReconciler) syncRBAC(ctx *chetypes.DeployContext) (bool, error) {
	if done, err := deploy.SyncClusterRoleToCluster(
		ctx,
		GetDevWorkspaceSccRbacResourcesName(),
		cb.getDevWorkspaceSccPolicyRules(ctx),
	); !done {
		return false, err
	}

	if crb, err := cb.getDevWorkspaceSccClusterRoleBindingSpec(ctx); err != nil {
		return false, err
	} else {
		if done, err := deploy.Sync(ctx, crb, deploy.ClusterRoleBindingDiffOpts); !done {
			return false, err
		}
	}

	if done, err := deploy.SyncClusterRoleToCluster(
		ctx,
		GetUserSccRbacResourcesName(),
		cb.getUserSccPolicyRules(ctx),
	); !done {
		return false, err
	}

	return true, nil
}

func (cb *ContainerBuildReconciler) removeSCC(ctx *chetypes.DeployContext) (bool, error) {
	sccName := constants.DefaultContainerBuildSccName
	if ctx.CheCluster.IsOpenShiftSecurityContextConstraintSet() {
		sccName = ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint
	}

	scc := &securityv1.SecurityContextConstraints{}
	if exists, err := deploy.GetClusterObject(ctx, sccName, scc); !exists {
		return err == nil, err
	}

	if scc.Labels[constants.KubernetesManagedByLabelKey] == deploy.GetManagedByLabel() {
		// Removes only if it is managed by operator
		return deploy.DeleteClusterObject(ctx, sccName, &securityv1.SecurityContextConstraints{})
	}

	return true, nil
}

func (cb *ContainerBuildReconciler) removeRBAC(ctx *chetypes.DeployContext) (bool, error) {
	if done, err := deploy.DeleteClusterObject(ctx, GetDevWorkspaceSccRbacResourcesName(), &rbacv1.ClusterRole{}); !done {
		return false, err
	}

	if done, err := deploy.DeleteClusterObject(ctx, GetDevWorkspaceSccRbacResourcesName(), &rbacv1.ClusterRoleBinding{}); !done {
		return false, err
	}

	if done, err := deploy.DeleteClusterObject(ctx, GetUserSccRbacResourcesName(), &rbacv1.ClusterRole{}); !done {
		return false, err
	}

	return true, nil
}

func (cb *ContainerBuildReconciler) getFinalizerName() string {
	return "container-build.finalizers.che.eclipse.org"
}

func (cb *ContainerBuildReconciler) getDevWorkspaceSccPolicyRules(ctx *chetypes.DeployContext) []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"get", "update", "use"},
			ResourceNames: []string{ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint},
		},
	}
}

func (cb *ContainerBuildReconciler) getDevWorkspaceSccClusterRoleBindingSpec(ctx *chetypes.DeployContext) (*rbacv1.ClusterRoleBinding, error) {
	devWorkspaceServiceAccountNamespace, err := cb.getDevWorkspaceServiceAccountNamespace(ctx)
	if devWorkspaceServiceAccountNamespace == "" {
		return nil, err
	}

	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   GetDevWorkspaceSccRbacResourcesName(),
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      constants.DevWorkspaceServiceAccountName,
				Namespace: devWorkspaceServiceAccountNamespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     GetDevWorkspaceSccRbacResourcesName(),
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
		},
	}, nil
}

func (cb *ContainerBuildReconciler) getDevWorkspaceServiceAccountNamespace(ctx *chetypes.DeployContext) (string, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	if exists, err := deploy.GetClusterObject(ctx, GetDevWorkspaceSccRbacResourcesName(), crb); err != nil {
		return "", err
	} else if exists {
		return crb.Subjects[0].Namespace, nil
	} else {
		sas := &corev1.ServiceAccountList{}
		if err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), sas); err != nil {
			return "", err
		}

		for _, sa := range sas.Items {
			if sa.Name == constants.DevWorkspaceServiceAccountName {
				return sa.Namespace, nil
			}
		}
	}

	return "", fmt.Errorf("ServiceAccount %s not found", constants.DevWorkspaceServiceAccountName)
}

func (cb *ContainerBuildReconciler) getUserSccPolicyRules(ctx *chetypes.DeployContext) []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
			ResourceNames: []string{ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint},
		},
	}
}

func (cb *ContainerBuildReconciler) getSccSpec(ctx *chetypes.DeployContext) *securityv1.SecurityContextConstraints {
	return &securityv1.SecurityContextConstraints{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SecurityContextConstraints",
			APIVersion: securityv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint,
			Labels: deploy.GetLabels(defaults.GetCheFlavor()),
		},
		AllowHostDirVolumePlugin: false,
		AllowHostIPC:             false,
		AllowHostNetwork:         false,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		AllowPrivilegeEscalation: pointer.BoolPtr(true),
		AllowPrivilegedContainer: false,
		AllowedCapabilities:      []corev1.Capability{"SETUID", "SETGID"},
		DefaultAddCapabilities:   nil,
		FSGroup:                  securityv1.FSGroupStrategyOptions{Type: securityv1.FSGroupStrategyMustRunAs},
		ReadOnlyRootFilesystem:   false,
		RequiredDropCapabilities: []corev1.Capability{"KILL", "MKNOD"},
		RunAsUser:                securityv1.RunAsUserStrategyOptions{Type: securityv1.RunAsUserStrategyMustRunAsRange},
		SELinuxContext:           securityv1.SELinuxContextStrategyOptions{Type: securityv1.SELinuxStrategyMustRunAs},
		SupplementalGroups:       securityv1.SupplementalGroupsStrategyOptions{Type: securityv1.SupplementalGroupsStrategyRunAsAny},
		Users:                    []string{},
		Groups:                   []string{},
		Volumes: []securityv1.FSType{
			securityv1.FSTypeConfigMap,
			securityv1.FSTypeDownwardAPI,
			securityv1.FSTypeEmptyDir,
			securityv1.FSTypePersistentVolumeClaim,
			securityv1.FSProjected,
			securityv1.FSTypeSecret,
		},
	}
}

func GetUserSccRbacResourcesName() string {
	return defaults.GetCheFlavor() + "-user-container-build"
}

func GetDevWorkspaceSccRbacResourcesName() string {
	return "dev-workspace-container-build"
}
