//
// Copyright (c) 2021 Red Hat, Inc.
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSyncAdditionalCACertsConfigMapToCluster(t *testing.T) {
	cert1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cert1",
			Namespace:       "eclipse-che",
			ResourceVersion: "1",
			Labels: map[string]string{
				"app.kubernetes.io/component": "ca-bundle",
				"app.kubernetes.io/part-of":   "che.eclipse.org"},
		},
		Data: map[string]string{"a1": "b1"},
	}
	cert2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert2",
			Namespace: "eclipse-che",
			// Go client set up resource version 1 itself on object creation.
			// ResourceVersion: "1",
			Labels: map[string]string{
				"app.kubernetes.io/component": "ca-bundle",
				"app.kubernetes.io/part-of":   "che.eclipse.org"},
		},
		Data: map[string]string{"a2": "b2"},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	corev1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme, cert1)
	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	// check ca-cert-merged
	done, err := SyncAdditionalCACertsConfigMapToCluster(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	cacertMerged := &corev1.ConfigMap{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: CheAllCACertsConfigMapName, Namespace: "eclipse-che"}, cacertMerged)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}
	if cacertMerged.ObjectMeta.Annotations["che.eclipse.org/included-configmaps"] != "cert1-1" {
		t.Fatalf("Failed to sync config map")
	}

	// let's create another configmap
	err = cli.Create(context.TODO(), cert2)
	if err != nil {
		t.Fatalf("Failed to create config map: %v", err)
	}

	// check ca-cert-merged
	_, err = SyncAdditionalCACertsConfigMapToCluster(deployContext)
	if err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncAdditionalCACertsConfigMapToCluster(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	err = cli.Get(context.TODO(), types.NamespacedName{Name: CheAllCACertsConfigMapName, Namespace: "eclipse-che"}, cacertMerged)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}
	if cacertMerged.ObjectMeta.Annotations["che.eclipse.org/included-configmaps"] != "cert1-1.cert2-1" {
		t.Fatalf("Failed to sync config map")
	}
}
