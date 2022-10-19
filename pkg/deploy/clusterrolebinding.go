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
package deploy

import (
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var ClusterRoleBindingDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(rbac.ClusterRoleBinding{}, "TypeMeta", "ObjectMeta"),
}

func SyncClusterRoleBindingToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	clusterRoleName string) (bool, error) {

	crbSpec := getClusterRoleBindingSpec(deployContext, name, serviceAccountName, deployContext.CheCluster.Namespace, clusterRoleName)
	return Sync(deployContext, crbSpec, ClusterRoleBindingDiffOpts)
}

func SyncClusterRoleBindingAndAddFinalizerToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	clusterRoleName string) (bool, error) {

	finalizer := GetFinalizerName(strings.ToLower(name) + ".crb")
	crbSpec := getClusterRoleBindingSpec(deployContext, name, serviceAccountName, deployContext.CheCluster.Namespace, clusterRoleName)
	return SyncAndAddFinalizer(deployContext, crbSpec, ClusterRoleBindingDiffOpts, finalizer)
}

func ReconcileClusterRoleBindingFinalizer(deployContext *chetypes.DeployContext, name string) error {
	if deployContext.CheCluster.DeletionTimestamp.IsZero() {
		return nil
	}

	finalizer := GetFinalizerName(strings.ToLower(name) + ".crb")
	return DeleteObjectWithFinalizer(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}, finalizer)
}

func GetLegacyUniqueClusterRoleBindingName(deployContext *chetypes.DeployContext, serviceAccount string, clusterRole string) string {
	return deployContext.CheCluster.Namespace + "-" + serviceAccount + "-" + clusterRole
}

func ReconcileLegacyClusterRoleBindingFinalizer(deployContext *chetypes.DeployContext, name string) error {
	if deployContext.CheCluster.DeletionTimestamp.IsZero() {
		return nil
	}

	finalizer := strings.ToLower(name) + ".clusterrolebinding.finalizers.che.eclipse.org"
	return DeleteObjectWithFinalizer(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{}, finalizer)
}

func getClusterRoleBindingSpec(
	deployContext *chetypes.DeployContext,
	name string,
	serviceAccountName string,
	serviceAccountNamespace string,
	clusterRoleName string) *rbac.ClusterRoleBinding {

	labels := GetLabels(defaults.GetCheFlavor())
	clusterRoleBinding := &rbac.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbac.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
			Annotations: map[string]string{
				constants.CheEclipseOrgNamespace: deployContext.CheCluster.Namespace,
			},
		},
		Subjects: []rbac.Subject{
			{
				Kind:      rbac.ServiceAccountKind,
				Name:      serviceAccountName,
				Namespace: serviceAccountNamespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Name:     clusterRoleName,
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
		},
	}

	return clusterRoleBinding
}
