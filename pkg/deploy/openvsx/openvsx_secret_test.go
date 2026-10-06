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

package openvsx

import (
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestOpenVSXSecretReconciler_CreatesSecretWhenEnabled(t *testing.T) {
	cheCtx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				Components: chev2.CheClusterComponents{
					OpenVSXRegistry: chev2.OpenVSXRegistry{
						Enable: true,
					},
				},
			},
		},
	).Build()

	reconciler := NewOpenVSXSecretReconciler()
	test.EnsureReconcile(t, cheCtx, reconciler.Reconcile)

	assert.True(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXCredentialsSecret, Namespace: "eclipse-che"}, &corev1.Secret{}))

	cheCtx.CheCluster.Spec.Components.OpenVSXRegistry.Enable = false
	test.EnsureReconcile(t, cheCtx, reconciler.Reconcile)

	assert.False(t, test.IsObjectExists(cheCtx.ClusterAPI.Client, types.NamespacedName{Name: constants.OpenVSXCredentialsSecret, Namespace: "eclipse-che"}, &corev1.Secret{}))
}
