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

package deploy

import (
	"context"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncRoleBindingToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncRoleBindingToCluster(ctx, "test", "sa", "clusterrole-1", "kind")
	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync a new role binding
	_, err = SyncRoleBindingToCluster(ctx, "test", "sa", "clusterrole-2", "kind")
	if err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync role binding twice to be sure update done correctly
	done, err = SyncRoleBindingToCluster(ctx, "test", "sa", "clusterrole-2", "kind")
	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	actual := &rbacv1.RoleBinding{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get crb: %v", err)
	}

	if actual.RoleRef.Name != "clusterrole-2" {
		t.Fatalf("Failed to sync crb: %v", err)
	}
}
