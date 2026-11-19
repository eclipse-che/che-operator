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

package pluginregistry

import (
	"os"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/test/checontext"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	"testing"
)

func TestShouldDeployPluginRegistryIfOpenVSXIsEmpty(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := checontext.NewBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				PluginRegistry: chev2.PluginRegistry{
					OpenVSXURL: ptr.To(""),
				},
			},
		},
	}).Build()

	pluginregistry := NewPluginRegistryReconciler()
	test.AssertReconciled(t, cheCtx, pluginregistry.Reconcile)

	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.Service{}))
	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.ConfigMap{}))
	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&appsv1.Deployment{}))
	assert.NotEmpty(t, cheCtx.CheCluster.Status.PluginRegistryURL)
}

func TestShouldDeployPluginRegistryIfOpenVSXIsEmptyByDefault(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	defaultOpenVSXURL := os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL")

	err := os.Unsetenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL")
	assert.NoError(t, err)

	defer func() {
		_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL", defaultOpenVSXURL)
	}()

	// re initialize defaults with new env var
	defaults.Initialize()

	cheCtx := checontext.NewBuilder().Build()

	pluginregistry := NewPluginRegistryReconciler()
	test.AssertReconciled(t, cheCtx, pluginregistry.Reconcile)

	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.Service{}))
	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.ConfigMap{}))
	assert.NoError(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&appsv1.Deployment{}))
	assert.NotEmpty(t, cheCtx.CheCluster.Status.PluginRegistryURL)
}

func TestShouldNotDeployPluginRegistryIfOpenVSXConfigured(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := checontext.NewBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				PluginRegistry: chev2.PluginRegistry{
					OpenVSXURL: ptr.To("https://openvsx.org"),
				},
			},
		},
	}).Build()

	pluginregistry := NewPluginRegistryReconciler()
	test.AssertReconciled(t, cheCtx, pluginregistry.Reconcile)

	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.Service{}))
	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.ConfigMap{}))
	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&appsv1.Deployment{}))
	assert.Empty(t, cheCtx.CheCluster.Status.PluginRegistryURL)
}

func TestShouldNotDeployPluginRegistryIfOpenVSXConfiguredByDefault(t *testing.T) {
	test.StartOn(infrastructure.OpenShiftV4)

	defaultOpenVSXURL := os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL")
	err := os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL", "https://openvsx.org")
	assert.NoError(t, err)

	defer func() {
		_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL", defaultOpenVSXURL)
	}()

	// re initialize defaults with new env var
	defaults.Initialize()

	cheCtx := checontext.NewBuilder().Build()

	pluginregistry := NewPluginRegistryReconciler()
	test.AssertReconciled(t, cheCtx, pluginregistry.Reconcile)

	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.Service{}))
	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&corev1.ConfigMap{}))
	assert.Error(t, cheCtx.ClusterAPI.Client.Get(
		cheCtx.Context,
		types.NamespacedName{Name: "plugin-registry", Namespace: "eclipse-che"},
		&appsv1.Deployment{}))
	assert.Empty(t, cheCtx.CheCluster.Status.PluginRegistryURL)
}
