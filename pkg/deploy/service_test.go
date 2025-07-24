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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestServiceToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncServiceToCluster(ctx, "test", []string{"port"}, []int32{8080}, "test")
	if !done || err != nil {
		t.Fatalf("Failed to sync service: %v", err)
	}

	// sync another service
	done, err = SyncServiceToCluster(ctx, "test", []string{"port"}, []int32{9090}, "test")
	if !done || err != nil {
		t.Fatalf("Failed to sync service: %v", err)
	}

	actual := &corev1.Service{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if actual.Spec.Ports[0].Port != 9090 {
		t.Fatalf("Failed to sync service.")
	}
}
