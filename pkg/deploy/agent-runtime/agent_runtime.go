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
	"fmt"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	sandboxv1beta1 "sigs.k8s.io/agent-sandbox/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var logger = ctrl.Log.WithName("agent-runtimes")

type AgentRuntimesReconciler struct {
	reconciler.Reconcilable
}

func NewAgentRuntimesReconciler() *AgentRuntimesReconciler {
	return &AgentRuntimesReconciler{}
}

func (ar *AgentRuntimesReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsAgentRuntimesEnabled() {
		if done, err := ar.deleteSandbox(ctx); !done {
			return reconcile.Result{RequeueAfter: time.Second}, false, err
		}

		return reconcile.Result{}, true, nil
	}

	err := ar.syncSandbox(ctx)
	if err != nil {
		return reconcile.Result{RequeueAfter: time.Minute}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (ar *AgentRuntimesReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	done, err := ar.deleteSandbox(ctx)
	if err != nil {
		logger.Error(err, "Failed to delete managed sandbox")
	}

	return done

}
func (ar *AgentRuntimesReconciler) syncSandbox(ctx *chetypes.DeployContext) error {
	if !infrastructure.IsAgentSandboxEnabled(ctx.ClusterAPI.DiscoveryClient) {
		logger.Info("Agent runtimes are enabled but the agent-sandbox operator is not installed")
		return nil
	}

	sandbox, err := ar.getSandboxSpec(ctx)
	if err != nil {
		return fmt.Errorf("failed to get Sandbox spec: %w", err)
	}

	err = ctx.ClusterAPI.ClientWrapper.Sync(
		ctx.Context,
		sandbox,
		&k8sclient.SyncOptions{DiffOpts: diffs.Sandbox},
	)
	if err != nil {
		return fmt.Errorf("failed to sync Sandbox %s/%s: %w", sandbox.Namespace, sandbox.Name, err)
	}

	return nil
}

func (ar *AgentRuntimesReconciler) deleteSandbox(ctx *chetypes.DeployContext) (bool, error) {
	if !infrastructure.IsAgentSandboxEnabled(ctx.ClusterAPI.DiscoveryClient) {
		return true, nil
	}

	agentRuntimes := ctx.CheCluster.Spec.AgentRuntimes
	if agentRuntimes.Namespace == "" {
		return true, nil
	}

	err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		ctx.Context,
		types.NamespacedName{Name: getSandboxName(), Namespace: agentRuntimes.Namespace},
		&sandboxv1beta1.Sandbox{},
	)
	if err != nil {
		return false, fmt.Errorf("failed to delete sandbox %s/%s: %w", agentRuntimes.Namespace, getSandboxName(), err)
	}

	return true, nil
}

func (ar *AgentRuntimesReconciler) getSandboxSpec(ctx *chetypes.DeployContext) (*sandboxv1beta1.Sandbox, error) {
	agentRuntimes := ctx.CheCluster.Spec.AgentRuntimes

	if agentRuntimes.Image == "" {
		return nil, fmt.Errorf("Image is not specified")
	}
	if agentRuntimes.Namespace == "" {
		return nil, fmt.Errorf("Namespace is not specified")
	}

	sandbox := &sandboxv1beta1.Sandbox{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sandboxv1beta1.GroupVersion.String(),
			Kind:       "Sandbox",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      getSandboxName(),
			Namespace: agentRuntimes.Namespace,
			Labels:    deploy.GetLabels(constants.AgentRuntimesComponentName),
		},
		Spec: sandboxv1beta1.SandboxSpec{
			Lifecycle: sandboxv1beta1.Lifecycle{
				ShutdownPolicy: ptr.To(sandboxv1beta1.ShutdownPolicyDelete),
			},
			SandboxBlueprint: sandboxv1beta1.SandboxBlueprint{
				PodTemplate: sandboxv1beta1.PodTemplate{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    constants.AgentRuntimesComponentName,
								Image:   agentRuntimes.Image,
								Command: []string{"/bin/bash", "-c", "sleep infinity"},
							},
						},
					},
				},
			},
		},
	}

	if agentRuntimes.RuntimeClassName != "" {
		sandbox.Spec.PodTemplate.Spec.RuntimeClassName = new(agentRuntimes.RuntimeClassName)
	}

	deploy.EnsurePodSecurityStandards(
		&sandbox.Spec.PodTemplate.Spec,
		constants.DefaultSecurityContextRunAsUser,
		constants.DefaultSecurityContextFsGroup,
	)

	return sandbox, nil
}

func getSandboxName() string {
	return defaults.GetCheFlavor() + constants.AgentRuntimesComponentName
}
