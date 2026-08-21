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

package agentruntimes

import (
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	sandboxv1beta1 "sigs.k8s.io/agent-sandbox/api/v1beta1"
)

func TestReconcileAgentRuntimesCreatesSandbox(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			AgentRuntimes: &chev2.AgentRuntimes{
				Enabled:          ptr.To(true),
				Namespace:        "sandbox-ns",
				RuntimeClassName: "kata",
				Image:            "quay.io/test/agent-runtime:latest",
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	reconciler := NewAgentRuntimesReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	sandbox := &sandboxv1beta1.Sandbox{}
	err := ctx.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      getSandboxName(),
			Namespace: "sandbox-ns",
		},
		sandbox,
	)
	assert.NoError(t, err)
	assert.Equal(t, "quay.io/test/agent-runtime:latest", sandbox.Spec.PodTemplate.Spec.Containers[0].Image)
	assert.Equal(t, constants.AgentRuntimesComponentName, sandbox.Spec.PodTemplate.Spec.Containers[0].Name)
	assert.Equal(t, "kata", *sandbox.Spec.PodTemplate.Spec.RuntimeClassName)
	assert.Equal(t, sandboxv1beta1.ShutdownPolicyDelete, *sandbox.Spec.ShutdownPolicy)
}

func TestReconcileAgentRuntimesDisabledDeletesSandbox(t *testing.T) {
	existingSandbox := &sandboxv1beta1.Sandbox{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sandboxv1beta1.GroupVersion.String(),
			Kind:       "Sandbox",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getSandboxName(),
			Namespace: "sandbox-ns",
		},
	}

	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			AgentRuntimes: &chev2.AgentRuntimes{
				Enabled:   ptr.To(false),
				Namespace: "sandbox-ns",
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(existingSandbox).Build()

	reconciler := NewAgentRuntimesReconciler()
	test.EnsureReconcile(t, ctx, reconciler.Reconcile)

	sandboxKey := types.NamespacedName{Name: getSandboxName(), Namespace: "sandbox-ns"}
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, sandboxKey, &sandboxv1beta1.Sandbox{}))
}

func TestGetSandboxSpec(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			AgentRuntimes: &chev2.AgentRuntimes{
				Enabled:          ptr.To(true),
				Namespace:        "sandbox-ns",
				RuntimeClassName: "kata",
				Image:            "quay.io/test/agent-runtime:latest",
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	ar := NewAgentRuntimesReconciler()
	sandbox, err := ar.getSandboxSpec(ctx)
	assert.NoError(t, err)

	assert.Equal(t, defaults.GetCheFlavor()+constants.AgentRuntimesComponentName, sandbox.Name)
	assert.Equal(t, "sandbox-ns", sandbox.Namespace)
	assert.Equal(t, "quay.io/test/agent-runtime:latest", sandbox.Spec.PodTemplate.Spec.Containers[0].Image)
	assert.Equal(t, "kata", *sandbox.Spec.PodTemplate.Spec.RuntimeClassName)
	assert.NotNil(t, sandbox.Spec.PodTemplate.Spec.Containers[0].SecurityContext)
}

func TestGetSandboxSpecMissingImage(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			AgentRuntimes: &chev2.AgentRuntimes{
				Enabled:          ptr.To(true),
				Namespace:        "sandbox-ns",
				RuntimeClassName: "kata",
			},
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).Build()

	ar := NewAgentRuntimesReconciler()
	_, err := ar.getSandboxSpec(ctx)
	assert.Error(t, err)
}
