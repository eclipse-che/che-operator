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
package consolelink

import (
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	console "github.com/openshift/api/console/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeDiscovery "k8s.io/client-go/discovery/fake"

	"testing"
)

func TestReconcileConsoleLink(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}

	ctx := test.GetDeployContext(cheCluster, []runtime.Object{})
	ctx.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
		{
			APIResources: []metav1.APIResource{
				{Name: ConsoleLinksResourceName},
			},
		},
	}

	consolelink := NewConsoleLinkReconciler()
	_, done, err := consolelink.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: defaults.GetConsoleLinkName()}, &console.ConsoleLink{}))
	assert.True(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))

	// Initialize DeletionTimestamp => checluster is being deleted
	done = consolelink.Finalize(ctx)
	assert.True(t, done)

	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: defaults.GetConsoleLinkName()}, &console.ConsoleLink{}))
	assert.False(t, utils.Contains(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
}
