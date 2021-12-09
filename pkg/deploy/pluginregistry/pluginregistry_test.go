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
package pluginregistry

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"testing"
)

func TestPluginRegistryReconcile(t *testing.T) {
	util.IsOpenShift = true
	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{})

	pluginregistry := NewPluginRegistryReconciler()
	_, done, err := pluginregistry.Reconcile(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, &corev1.Service{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, &routev1.Route{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, &corev1.ConfigMap{}))
	assert.True(t, util.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"}, &appsv1.Deployment{}))
	assert.NotEmpty(t, ctx.CheCluster.Status.PluginRegistryURL)
}
