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
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncJobToCluster(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := SyncJobToCluster(ctx, "test", "component", "image-1", "sa", map[string]string{})
	if !done || err != nil {
		t.Fatalf("Failed to sync job: %v", err)
	}

	done, err = SyncJobToCluster(ctx, "test", "component", "image-2", "sa", map[string]string{})
	if !done || err != nil {
		t.Fatalf("Failed to sync job: %v", err)
	}

	actual := &batchv1.Job{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if actual.Spec.Template.Spec.Containers[0].Image != "image-2" {
		t.Fatalf("Failed to sync job")
	}
}
