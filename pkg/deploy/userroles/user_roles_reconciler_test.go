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

package userroles

import (
	"fmt"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newTestCheCluster(namespace string) *chev2.CheCluster {
	return &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: namespace,
		},
	}
}

// TestReconcileCreatesClusterRoles verifies that Reconcile creates the expected
// ClusterRoles and ClusterRoleBindings for the given namespace.
func TestReconcileCreatesClusterRoles(t *testing.T) {
	namespace := "eclipse-che"
	cr := newTestCheCluster(namespace)
	ctx := test.NewCtxBuilder().WithCheCluster(cr).Build()

	reconciler := NewUserRolesReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.NoError(t, err)
	assert.True(t, done)

	commonName := fmt.Sprintf(userCommonPermissionsTemplateName, namespace)
	dwName := fmt.Sprintf(userDWPermissionsTemplateName, namespace)

	// ClusterRoles must exist
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: commonName}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwName}, &rbacv1.ClusterRole{}))

	// ClusterRoleBindings must exist
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: commonName}, &rbacv1.ClusterRoleBinding{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwName}, &rbacv1.ClusterRoleBinding{}))
}

// TestReconcileAppendsFinalizer verifies that Reconcile appends the
// userRolesFinalizerName to the CheCluster resource after a successful sync.
func TestReconcileAppendsFinalizer(t *testing.T) {
	namespace := "eclipse-che"
	cr := newTestCheCluster(namespace)
	ctx := test.NewCtxBuilder().WithCheCluster(cr).Build()

	reconciler := NewUserRolesReconciler()
	_, done, err := reconciler.Reconcile(ctx)

	assert.NoError(t, err)
	assert.True(t, done)

	finalizerFound := false
	for _, f := range ctx.CheCluster.Finalizers {
		if f == userRolesFinalizerName {
			finalizerFound = true
			break
		}
	}
	assert.True(t, finalizerFound, "expected finalizer %q to be appended", userRolesFinalizerName)
}

// TestFinalizeDeletesClusterRoles verifies that Finalize removes the ClusterRoles
// and ClusterRoleBindings created by Reconcile and removes the finalizer.
func TestFinalizeDeletesClusterRoles(t *testing.T) {
	namespace := "eclipse-che"
	cr := newTestCheCluster(namespace)
	ctx := test.NewCtxBuilder().WithCheCluster(cr).Build()

	reconciler := NewUserRolesReconciler()

	// First reconcile to create resources and add finalizer
	_, done, err := reconciler.Reconcile(ctx)
	assert.NoError(t, err)
	assert.True(t, done)

	commonName := fmt.Sprintf(userCommonPermissionsTemplateName, namespace)
	dwName := fmt.Sprintf(userDWPermissionsTemplateName, namespace)

	// Resources should exist after Reconcile
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: commonName}, &rbacv1.ClusterRole{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwName}, &rbacv1.ClusterRole{}))

	// Finalize should delete them
	finalizeDone := reconciler.Finalize(ctx)
	assert.True(t, finalizeDone)

	// ClusterRoles should be gone
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: commonName}, &rbacv1.ClusterRole{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwName}, &rbacv1.ClusterRole{}))

	// ClusterRoleBindings should be gone
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: commonName}, &rbacv1.ClusterRoleBinding{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: dwName}, &rbacv1.ClusterRoleBinding{}))
}

// TestGetDefaultUserClusterRoles verifies that helper returns the two expected role names.
func TestGetDefaultUserClusterRoles(t *testing.T) {
	namespace := "test-ns"
	names := GetDefaultUserClusterRoles(namespace)

	assert.Len(t, names, 2)
	assert.Contains(t, names, fmt.Sprintf(userCommonPermissionsTemplateName, namespace))
	assert.Contains(t, names, fmt.Sprintf(userDWPermissionsTemplateName, namespace))
}
