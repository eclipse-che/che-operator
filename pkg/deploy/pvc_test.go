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

	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncPVCToCluster(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
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

	done, err := SyncPVCToCluster(deployContext, "test", "1Gi", "che")
	if !done || err != nil {
		t.Fatalf("Failed to sync pvc: %v", err)
	}

	// sync a new pvc
	_, err = SyncPVCToCluster(deployContext, "test", "2Gi", "che")
	if err != nil {
		t.Fatalf("Failed to sync pvc: %v", err)
	}

	// sync pvc twice to be sure update done correctly
	done, err = SyncPVCToCluster(deployContext, "test", "2Gi", "che")
	if !done || err != nil {
		t.Fatalf("Failed to sync pvc: %v", err)
	}

	actual := &corev1.PersistentVolumeClaim{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get pvc: %v", err)
	}

	if !actual.Spec.Resources.Requests[corev1.ResourceStorage].Equal(resource.MustParse("2Gi")) {
		t.Fatalf("Failed to sync pvc: %v", err)
	}
}
