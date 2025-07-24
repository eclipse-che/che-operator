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

func TestSyncClusterRoleBindingToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncClusterRoleBindingToCluster(ctx, "test", "sa", "clusterrole-1")
	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync a new cluster role binding
	_, err = SyncClusterRoleBindingToCluster(ctx, "test", "sa", "clusterrole-2")
	if err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncClusterRoleBindingToCluster(ctx, "test", "sa", "clusterrole-2")
	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	actual := &rbacv1.ClusterRoleBinding{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test"}, actual)
	if err != nil {
		t.Fatalf("Failed to get crb: %v", err)
	}

	if actual.RoleRef.Name != "clusterrole-2" {
		t.Fatalf("Failed to sync crb: %v", err)
	}
}
