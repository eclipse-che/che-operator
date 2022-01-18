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
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	console "github.com/openshift/api/console/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeDiscovery "k8s.io/client-go/discovery/fake"

	"testing"
)

func TestReconcileConsoleLink(t *testing.T) {
	cheCluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{},
	}

	util.IsOpenShift4 = true
	ctx := deploy.GetTestDeployContext(cheCluster, []runtime.Object{})
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

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: deploy.DefaultConsoleLinkName()}, &console.ConsoleLink{}))
	assert.True(t, util.ContainsString(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))

	// Initialize DeletionTimestamp => checluster is being deleted
	done = consolelink.Finalize(ctx)
	assert.True(t, done)

	assert.False(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: deploy.DefaultConsoleLinkName()}, &console.ConsoleLink{}))
	assert.False(t, util.ContainsString(ctx.CheCluster.Finalizers, ConsoleLinkFinalizerName))
}
