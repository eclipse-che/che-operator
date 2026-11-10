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

package che

import (
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// devWorkspaceOperatorPodPredicates lets through only the DevWorkspace Operator controller pod events
// that can change which pod the project clone image is read from.
var devWorkspaceOperatorPodPredicates = predicate.Funcs{
	CreateFunc:  func(e event.CreateEvent) bool { return isDevWorkspaceOperatorPod(e.Object) },
	DeleteFunc:  func(e event.DeleteEvent) bool { return isDevWorkspaceOperatorPod(e.Object) },
	GenericFunc: func(e event.GenericEvent) bool { return isDevWorkspaceOperatorPod(e.Object) },
	UpdateFunc: func(e event.UpdateEvent) bool {
		oldPod, ok := e.ObjectOld.(*corev1.Pod)
		if !ok {
			return false
		}

		newPod, ok := e.ObjectNew.(*corev1.Pod)
		if !ok || !isDevWorkspaceOperatorPod(newPod) {
			return false
		}

		return oldPod.Status.Phase != newPod.Status.Phase ||
			oldPod.DeletionTimestamp.IsZero() != newPod.DeletionTimestamp.IsZero()
	},
}

// inNamespaceEventFilter lets through events of objects in the watched namespace, and of the
// DevWorkspace Operator controller pods, which run in the DevWorkspace Operator namespace.
func inNamespaceEventFilter(namespace string) predicate.Predicate {
	return predicate.Or(
		utils.InNamespaceEventFilter(namespace),
		predicate.NewPredicateFuncs(isDevWorkspaceOperatorPod),
	)
}

func isDevWorkspaceOperatorPod(obj client.Object) bool {
	_, ok := obj.(*corev1.Pod)
	return ok && deploy.IsDevWorkspaceComponent(obj.GetLabels())
}
