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

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncConfigMapDataToCluster(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: chetypes.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	done, err := SyncConfigMapDataToCluster(deployContext, "test", map[string]string{"a": "b"}, "che")
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync a new config map
	_, err = SyncConfigMapDataToCluster(deployContext, "test", map[string]string{"c": "d"}, "che")
	if err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncConfigMapDataToCluster(deployContext, "test", map[string]string{"c": "d"}, "che")
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	actual := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}

	if actual.Data["c"] != "d" {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	if actual.Data["a"] == "b" {
		t.Fatalf("Failed to sync config map: %v", err)
	}
}

func TestSyncConfigMapSpecDataToCluster(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: chetypes.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	spec := GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "che")
	done, err := SyncConfigMapSpecToCluster(deployContext, spec)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// check if labels
	spec = GetConfigMapSpec(deployContext, "test", map[string]string{"a": "b"}, "che")
	spec.ObjectMeta.Labels = map[string]string{"l": "v"}
	_, err = SyncConfigMapSpecToCluster(deployContext, spec)
	if err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncConfigMapSpecToCluster(deployContext, spec)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	actual := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}
	if actual.ObjectMeta.Labels["l"] != "v" {
		t.Fatalf("Failed to sync config map")
	}
}
