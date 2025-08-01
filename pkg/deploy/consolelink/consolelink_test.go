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

package consolelink

import (
	"context"
	"fmt"

	chev2 "github.com/eclipse-che/che-operator/api/v2"

	"testing"

	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	consolev1 "github.com/openshift/api/console/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReconcileConsoleLink(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	consolelink := NewConsoleLinkReconciler()
	test.EnsureReconcile(t, ctx, consolelink.Reconcile)

	consoleLink := &consolev1.ConsoleLink{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: defaults.GetConsoleLinkName()}, consoleLink)
	assert.Nil(t, err)
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
	assert.Equal(t, "https://che-host", consoleLink.Spec.Href)

	// Initialize DeletionTimestamp => checluster is being deleted
	done := consolelink.Finalize(ctx)
	assert.True(t, done)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: defaults.GetConsoleLinkName()}, &consolev1.ConsoleLink{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
}

func TestReconcileConsoleLinkWhenCheURLChanged(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://test-host",
		},
	}

	existedConsoleLink := &consolev1.ConsoleLink{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConsoleLink",
			APIVersion: consolev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaults.GetConsoleLinkName(),
		},
		Spec: consolev1.ConsoleLinkSpec{
			Link: consolev1.Link{
				Href: "https://che-host",
				Text: defaults.GetConsoleLinkDisplayName()},
			Location: consolev1.ApplicationMenu,
			ApplicationMenu: &consolev1.ApplicationMenuSpec{
				Section:  defaults.GetConsoleLinkSection(),
				ImageURL: fmt.Sprintf("https://%s%s", "che-host", defaults.GetConsoleLinkImage()),
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(existedConsoleLink).Build()

	consoleLinkReconciler := NewConsoleLinkReconciler()
	test.EnsureReconcile(t, ctx, consoleLinkReconciler.Reconcile)

	consoleLink := &consolev1.ConsoleLink{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: defaults.GetConsoleLinkName()}, consoleLink)
	assert.Nil(t, err)
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
	assert.Equal(t, "https://test-host", consoleLink.Spec.Href)
	assert.Equal(t, fmt.Sprintf("https://test-host%s", defaults.GetConsoleLinkImage()), consoleLink.Spec.ApplicationMenu.ImageURL)
}
