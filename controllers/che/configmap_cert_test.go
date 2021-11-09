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
	"context"

	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncTrustStoreConfigMapToCluster(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ServerTrustStoreConfigMapName: "trust",
				},
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	done, err := SyncTrustStoreConfigMapToCluster(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	actual := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "trust", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}
	if actual.ObjectMeta.Labels[injector] != "true" {
		t.Fatalf("Failed to sync config map")
	}
}

func TestSyncExistedTrustStoreConfigMapToCluster(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
			Spec: orgv1.CheClusterSpec{
				Server: orgv1.CheClusterSpecServer{
					ServerTrustStoreConfigMapName: "trust",
				},
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trust",
			Namespace: "eclipse-che",
			Labels:    map[string]string{"a": "b"},
		},
		Data: map[string]string{"d": "c"},
	}
	err := cli.Create(context.TODO(), cm)
	if err != nil {
		t.Fatalf("Failed to create config map: %v", err)
	}

	done, err := SyncTrustStoreConfigMapToCluster(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	actual := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "trust", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}
	if actual.ObjectMeta.Labels[injector] != "true" || actual.ObjectMeta.Labels["a"] != "b" {
		t.Fatalf("Failed to sync config map")
	}
	if actual.Data["d"] != "c" {
		t.Fatalf("Failed to sync config map")
	}
}
