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

package migration

import (
	"context"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUserRolesMigratorDeletesLegacyRoleBindings(t *testing.T) {
	cheNs := "eclipse-che"
	userNs := "user-namespace"

	// Legacy RoleBinding (no part-of label) - should be deleted
	legacyRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-rb",
			Namespace: userNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     cheNs + "-cheworkspaces-clusterrole",
		},
	}

	// Legacy DW RoleBinding (no part-of label) - should be deleted
	legacyDWRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-dw-rb",
			Namespace: userNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     cheNs + "-cheworkspaces-devworkspace-clusterrole",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(legacyRB, legacyDWRB).Build()

	migrator := NewUserRolesMigrator()
	result, done, err := migrator.Reconcile(ctx)
	assert.NoError(t, err)
	assert.True(t, done)
	assert.Equal(t, false, result.Requeue)

	// Verify legacy RoleBindings were deleted
	rbList := &rbacv1.RoleBindingList{}
	err = ctx.ClusterAPI.NonCachingClient.List(context.TODO(), rbList, &client.ListOptions{})
	assert.NoError(t, err)
	assert.Empty(t, rbList.Items, "legacy RoleBindings should have been deleted")

	// Second reconcile should be no-op (migrationDone=true)
	_, done2, err2 := migrator.Reconcile(ctx)
	assert.NoError(t, err2)
	assert.True(t, done2)
}

func TestUserRolesMigratorPreservesOperatorOwnedRoleBindings(t *testing.T) {
	cheNs := "eclipse-che"
	userNs := "user-namespace"

	// Operator-owned RoleBinding (has part-of label) - should be preserved
	operatorRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "operator-rb",
			Namespace: userNs,
			Labels: map[string]string{
				constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     cheNs + "-cheworkspaces-clusterrole",
		},
	}

	// Unrelated RoleBinding - should be preserved
	unrelatedRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-rb",
			Namespace: userNs,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "some-other-clusterrole",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(operatorRB, unrelatedRB).Build()

	migrator := NewUserRolesMigrator()
	_, done, err := migrator.Reconcile(ctx)
	assert.NoError(t, err)
	assert.True(t, done)

	// Verify both RoleBindings are still present
	rbList := &rbacv1.RoleBindingList{}
	err = ctx.ClusterAPI.NonCachingClient.List(context.TODO(), rbList, &client.ListOptions{})
	assert.NoError(t, err)
	assert.Len(t, rbList.Items, 2, "operator-owned and unrelated RoleBindings should be preserved")
}

func TestUserRolesMigratorFinalize(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()
	migrator := NewUserRolesMigrator()
	assert.True(t, migrator.Finalize(ctx))
}
