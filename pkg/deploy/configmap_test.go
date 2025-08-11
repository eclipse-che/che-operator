//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncConfigMapDataToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncConfigMapDataToCluster(ctx, "test", map[string]string{"a": "b"}, "che")
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync a new config map
	_, err = SyncConfigMapDataToCluster(ctx, "test", map[string]string{"c": "d"}, "che")
	if err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncConfigMapDataToCluster(ctx, "test", map[string]string{"c": "d"}, "che")
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	actual := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
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
	ctx := test.NewCtxBuilder().Build()

	spec := InitConfigMap(ctx, "test", map[string]string{"a": "b"}, "che")
	done, err := SyncConfigMapSpecToCluster(ctx, spec)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// check if labels
	spec = InitConfigMap(ctx, "test", map[string]string{"a": "b"}, "che")
	spec.ObjectMeta.Labels = map[string]string{"l": "v"}
	_, err = SyncConfigMapSpecToCluster(ctx, spec)
	if err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncConfigMapSpecToCluster(ctx, spec)
	if !done || err != nil {
		t.Fatalf("Failed to sync config map: %v", err)
	}

	actual := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get config map: %v", err)
	}
	if actual.ObjectMeta.Labels["l"] != "v" {
		t.Fatalf("Failed to sync config map")
	}
}
