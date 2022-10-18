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
	if exists, err := deploy.GetClusterObject(
		ctx,
		ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint,
		&securityv1.SecurityContextConstraints{},
	); err != nil {
		return false, nil
	} else if exists {
		// Don't override existed SCC
		return true, nil
	}

	return deploy.Sync(ctx, cb.getSCCSpec(ctx))
}

func (cb *ContainerBuildReconciler) syncRBAC(ctx *chetypes.DeployContext) (bool, error) {
	if done, err := deploy.SyncClusterRoleToCluster(ctx, cb.getClusterRoleName(), cb.getPolicyRules(ctx)); !done {
		return false, err
	}

	if devWorkspaceServiceAccountNamespace, err := cb.getDevWorkspaceServiceAccountNamespace(ctx); devWorkspaceServiceAccountNamespace == "" {
		return false, err
	} else {
		return deploy.SyncClusterRoleBindingToClusterInGivenNamespace(
			ctx,
			cb.getClusterRoleBindingName(),
			constants.DevWorkspaceServiceAccountName,
			devWorkspaceServiceAccountNamespace,
			cb.getClusterRoleName())
	}
}

func (cb *ContainerBuildReconciler) getDevWorkspaceServiceAccountNamespace(ctx *chetypes.DeployContext) (string, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	if exists, err := deploy.GetClusterObject(ctx, cb.getClusterRoleBindingName(), crb); err != nil {
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

func (cb *ContainerBuildReconciler) removeSCC(ctx *chetypes.DeployContext) (bool, error) {
	if ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration == nil {
		return true, nil
	}

	sccName := ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint
	if sccName != "" {
		scc := &securityv1.SecurityContextConstraints{}
		if exists, err := deploy.GetClusterObject(ctx, sccName, scc); !exists {
			return err == nil, err
		}

		if scc.Labels[constants.KubernetesManagedByLabelKey] == deploy.GetManagedByLabel() {
			// Removes only if it is managed by operator
			return deploy.DeleteClusterObject(ctx, sccName, &securityv1.SecurityContextConstraints{})
		}
	}

	return true, nil
}

func (cb *ContainerBuildReconciler) removeRBAC(ctx *chetypes.DeployContext) (bool, error) {
	if done, err := deploy.DeleteClusterObject(ctx, cb.getClusterRoleName(), &rbacv1.ClusterRole{}); !done {
		return false, err
	}

	if done, err := deploy.DeleteClusterObject(ctx, cb.getClusterRoleBindingName(), &rbacv1.ClusterRoleBinding{}); !done {
		return false, err
	}

	return true, nil
}

func (cb *ContainerBuildReconciler) getClusterRoleName() string {
	return defaults.GetCheFlavor() + "-container-build-scc"
}
func (cb *ContainerBuildReconciler) getClusterRoleBindingName() string {
	return defaults.GetCheFlavor() + "-container-build-scc"
}

func (cb *ContainerBuildReconciler) getFinalizerName() string {
	return "container-build.finalizers.che.eclipse.org"
}

func (cb *ContainerBuildReconciler) getPolicyRules(ctx *chetypes.DeployContext) []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			ResourceNames: []string{ctx.CheCluster.Spec.DevEnvironments.ContainerBuildConfiguration.OpenShiftSecurityContextConstraint},
			Verbs:         []string{"get", "update"},
		},
	}
}

func (cb *ContainerBuildReconciler) getSCCSpec(ctx *chetypes.DeployContext) *securityv1.SecurityContextConstraints {
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
		// Temporary workaround for https://github.com/devfile/devworkspace-operator/issues/884
		Priority:                 pointer.Int32Ptr(20),
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
