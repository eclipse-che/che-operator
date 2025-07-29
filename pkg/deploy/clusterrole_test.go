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

func TestSyncClusterRole(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncClusterRoleToCluster(ctx, "test", []rbacv1.PolicyRule{
		{
			APIGroups: []string{"test-1"},
			Resources: []string{"test-1"},
			Verbs:     []string{"test-1"},
		},
	})

	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync a new cluster role
	_, err = SyncClusterRoleToCluster(ctx, "test", []rbacv1.PolicyRule{
		{
			APIGroups: []string{"test-2"},
			Resources: []string{"test-2"},
			Verbs:     []string{"test-2"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to cluster role: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncClusterRoleToCluster(ctx, "test", []rbacv1.PolicyRule{
		{
			APIGroups: []string{"test-2"},
			Resources: []string{"test-2"},
			Verbs:     []string{"test-2"},
		},
	})
	if !done || err != nil {
		t.Fatalf("Failed to cluster role: %v", err)
	}

	actual := &rbacv1.ClusterRole{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test"}, actual)
	if err != nil {
		t.Fatalf("Failed to get cluster role: %v", err)
	}

	if actual.Rules[0].Resources[0] != "test-2" {
		t.Fatalf("Failed to sync cluster role: %v", err)
	}
}
