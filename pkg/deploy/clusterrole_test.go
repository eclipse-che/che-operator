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
	"context"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestSyncClusterRole(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	rbacv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	done, err := SyncClusterRoleToCluster(deployContext, "test", []rbacv1.PolicyRule{
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
	_, err = SyncClusterRoleToCluster(deployContext, "test", []rbacv1.PolicyRule{
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
	done, err = SyncClusterRoleToCluster(deployContext, "test", []rbacv1.PolicyRule{
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
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test"}, actual)
	if err != nil {
		t.Fatalf("Failed to get cluster role: %v", err)
	}

	if actual.Rules[0].Resources[0] != "test-2" {
		t.Fatalf("Failed to sync cluster role: %v", err)
	}
}
