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

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestSyncClusterRoleBindingToCluster(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	rbacv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		ClusterAPI: chetypes.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	done, err := SyncClusterRoleBindingToCluster(deployContext, "test", "sa", "clusterrole-1")
	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync a new cluster role binding
	_, err = SyncClusterRoleBindingToCluster(deployContext, "test", "sa", "clusterrole-2")
	if err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncClusterRoleBindingToCluster(deployContext, "test", "sa", "clusterrole-2")
	if !done || err != nil {
		t.Fatalf("Failed to sync crb: %v", err)
	}

	actual := &rbacv1.ClusterRoleBinding{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test"}, actual)
	if err != nil {
		t.Fatalf("Failed to get crb: %v", err)
	}

	if actual.RoleRef.Name != "clusterrole-2" {
		t.Fatalf("Failed to sync crb: %v", err)
	}
}
