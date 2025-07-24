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

func TestSyncRoleToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncRoleToCluster(ctx, "test", []rbacv1.PolicyRule{
		{
			APIGroups: []string{"test-1"},
			Resources: []string{"test-1"},
			Verbs:     []string{"test-1"},
		},
	})
	if !done || err != nil {
		t.Fatalf("Failed to sync role: %v", err)
	}

	done, err = SyncRoleToCluster(ctx, "test", []rbacv1.PolicyRule{
		{
			APIGroups: []string{"test-2"},
			Resources: []string{"test-2"},
			Verbs:     []string{"test-2"},
		},
	})

	actual := &rbacv1.Role{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get role: %v", err)
	}

	if actual.Rules[0].Resources[0] != "test-2" {
		t.Fatalf("Failed to sync role: %v", err)
	}
}
