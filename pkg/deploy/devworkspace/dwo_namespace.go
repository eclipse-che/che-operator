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

package devworkspace

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type DwoNamespaceReconciler struct {
	reconciler.Reconcilable
}

func NewDwoNamespaceReconciler() *DwoNamespaceReconciler {
	return &DwoNamespaceReconciler{}
}

func (r *DwoNamespaceReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	dwoNamespace, err := r.getDevWorkspaceNamespace(ctx)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	ctx.DwoNamespace = dwoNamespace
	return reconcile.Result{}, true, nil
}

func (r *DwoNamespaceReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

// getDevWorkspaceNamespace returns the namespace of the DevWorkspace operator.
// It searches for the DevWorkspace Operator Pods by its labels.
func (r *DwoNamespaceReconciler) getDevWorkspaceNamespace(ctx *chetypes.DeployContext) (string, error) {
	selector := labels.SelectorFromSet(
		labels.Set{
			constants.KubernetesNameLabelKey:   constants.DevWorkspaceControllerName,
			constants.KubernetesPartOfLabelKey: constants.DevWorkspaceOperatorName,
		},
	)

	items, err := ctx.ClusterAPI.NonCachingClientWrapper.List(
		context.TODO(),
		&corev1.PodList{},
		&client.ListOptions{LabelSelector: selector},
	)
	if err != nil {
		return "", err
	}

	for _, item := range items {
		pod := item.(*corev1.Pod)
		if pod.Spec.ServiceAccountName == constants.DevWorkspaceServiceAccountName {
			return pod.Namespace, nil
		}
	}

	return "", fmt.Errorf("DevWorkspace namespace not found")
}
