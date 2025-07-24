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
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	finalizer = "some.finalizer"
)

func TestAppendFinalizer(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := AppendFinalizer(ctx, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if !utils.Contains(ctx.CheCluster.ObjectMeta.Finalizers, finalizer) {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	// shouldn't add finalizer twice
	err = AppendFinalizer(ctx, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if len(ctx.CheCluster.ObjectMeta.Finalizers) != 1 {
		t.Fatalf("Finalizer shouldn't be added twice")
	}
}

func TestDeleteFinalizer(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "eclipse-che",
			Name:       "eclipse-che",
			Finalizers: []string{finalizer},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	err := DeleteFinalizer(ctx, finalizer)
	if err != nil {
		t.Fatalf("Failed to append finalizer: %v", err)
	}

	if utils.Contains(ctx.CheCluster.ObjectMeta.Finalizers, finalizer) {
		t.Fatalf("Failed to delete finalizer: %v", err)
	}
}
