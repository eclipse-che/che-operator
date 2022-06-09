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
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncPVCToCluster(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	done, err := SyncPVCToCluster(ctx, "test", &chev2.PVC{ClaimSize: "1Gi"}, "che")
	assert.True(t, done)
	assert.Nil(t, err)

	// sync a new pvc
	_, err = SyncPVCToCluster(ctx, "test", &chev2.PVC{ClaimSize: "2Gi"}, "che")
	assert.Nil(t, err)

	// sync pvc twice to be sure update done correctly
	_, err = SyncPVCToCluster(ctx, "test", &chev2.PVC{ClaimSize: "2Gi"}, "che")
	assert.Nil(t, err)

	actual := &corev1.PersistentVolumeClaim{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	assert.Nil(t, err)
	assert.Equal(t, actual.Spec.Resources.Requests[corev1.ResourceStorage], resource.MustParse("2Gi"))
}
