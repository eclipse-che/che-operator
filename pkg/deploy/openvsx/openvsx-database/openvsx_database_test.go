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

package openvsx_database

import (
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/test/checontext"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestOpenVSXDatabaseReconciler(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := checontext.NewBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: ptr.To(true),
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	test.AssertReconciled(t, cheCtx, reconciler.Reconcile)

	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"},
		&appsv1.Deployment{}))
	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"},
		&corev1.Service{}))
	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"},
		&corev1.PersistentVolumeClaim{}))

	cheCtx.CheCluster.Spec.Components.OpenVSXRegistry.Enable = ptr.To(false)
	test.AssertReconciled(t, cheCtx, reconciler.Reconcile)

	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"},
		&appsv1.Deployment{}))
	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"},
		&corev1.Service{}))
	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"},
		&corev1.PersistentVolumeClaim{}))
}
