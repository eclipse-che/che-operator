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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestServiceSpec(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				OpenVSXRegistry: chev2.OpenVSXRegistry{
					Enabled: ptr.To(true),
				},
			},
		},
	}).Build()

	reconciler := NewOpenVSXDatabaseReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	service := &corev1.Service{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseComponentName, Namespace: "eclipse-che"}, service)
	assert.NoError(t, err)

	assert.Equal(t, constants.OpenVSXDatabaseServicePort, service.Spec.Ports[0].Port)
	assert.Equal(t, corev1.ProtocolTCP, service.Spec.Ports[0].Protocol)
	assert.Equal(t, constants.OpenVSXDatabaseComponentName, service.Spec.Ports[0].Name)
}
