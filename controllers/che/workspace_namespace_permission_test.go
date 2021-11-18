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
package che

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileWorkspacePermissions(t *testing.T) {
	deployContext := deploy.GetTestDeployContext(nil, []runtime.Object{})
	reconciler := &CheClusterReconciler{
		client:          deployContext.ClusterAPI.Client,
		nonCachedClient: deployContext.ClusterAPI.Client,
		discoveryClient: deployContext.ClusterAPI.DiscoveryClient,
		Scheme:          deployContext.ClusterAPI.Scheme,
		tests:           true,
	}

	done, err := reconciler.reconcileWorkspacePermissions(deployContext)
	if err != nil {
		t.Fatalf("Failed to reconcile permissions: %v", err)
	}
	if !done {
		t.Fatalf("Permissions are not reconciled.")
	}

	if !util.ContainsString(deployContext.CheCluster.Finalizers, CheWorkspacesClusterPermissionsFinalizerName) {
		t.Fatalf("Finalizer '%s' not added", CheWorkspacesClusterPermissionsFinalizerName)
	}
	if !util.ContainsString(deployContext.CheCluster.Finalizers, NamespacesEditorPermissionsFinalizerName) {
		t.Fatalf("Finalizer '%s' not added", NamespacesEditorPermissionsFinalizerName)
	}
	if !util.ContainsString(deployContext.CheCluster.Finalizers, DevWorkspacePermissionsFinalizerName) {
		t.Fatalf("Finalizer '%s' not added", DevWorkspacePermissionsFinalizerName)
	}

	name := "eclipse-che-cheworkspaces-clusterrole"
	exists, _ := deploy.Get(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRole{})
	if !exists {
		t.Fatalf("Cluster Role '%s' not found", name)
	}
	exists, _ = deploy.Get(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{})
	if !exists {
		t.Fatalf("Cluster Role Binding '%s' not found", name)
	}

	name = "eclipse-che-cheworkspaces-namespaces-clusterrole"
	exists, _ = deploy.Get(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRole{})
	if !exists {
		t.Fatalf("Cluster Role '%s' not found", name)
	}
	exists, _ = deploy.Get(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{})
	if !exists {
		t.Fatalf("Cluster Role Binding '%s' not found", name)
	}

	name = "eclipse-che-cheworkspaces-devworkspace-clusterrole"
	exists, _ = deploy.Get(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRole{})
	if !exists {
		t.Fatalf("Cluster Role '%s' not found", name)
	}
	exists, _ = deploy.Get(deployContext, types.NamespacedName{Name: name}, &rbac.ClusterRoleBinding{})
	if !exists {
		t.Fatalf("Cluster Role Binding '%s' not found", name)
	}
}
