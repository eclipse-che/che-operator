//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package database

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func TestPVCSync(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()
	reconciler := NewOpenVSXDatabaseReconciler()

	err := reconciler.syncPVC(ctx)
	assert.NoError(t, err)

	ctx.CheCluster.Spec.Components.OpenVSXRegistry = chev2.OpenVSXRegistry{
		Database: &chev2.OpenVSXDatabase{
			Storage: &chev2.PVC{
				ClaimSize: "4Gi",
			},
		},
	}

	pvc := &corev1.PersistentVolumeClaim{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, pvc)
	assert.NoError(t, err)
	assert.Equal(t, resource.MustParse(constants.OpenVSXDatabaseClaimSize), pvc.Spec.Resources.Requests[corev1.ResourceStorage])

	err = reconciler.syncPVC(ctx)
	assert.NoError(t, err)

	pvc = &corev1.PersistentVolumeClaim{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, pvc)
	assert.NoError(t, err)
	assert.Equal(t, resource.MustParse("4Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
	assert.NotEqual(t, "4Gi", constants.OpenVSXDatabaseClaimSize)
}
