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

package usernamespace

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/deploy/devworkspace"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *CheUserNamespaceReconciler) watchRuleForDevWorkspacePod() handler.EventHandler {
	// Handle the create event to discover the DevWorkspace Operator namespace.
	// Log an error if multiple DevWorkspace Operators are found running in different namespaces, as this indicates an invalid deployment.
	return handler.Funcs{
		CreateFunc: func(
			ctx context.Context,
			evt event.CreateEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			pod, ok := evt.Object.(*corev1.Pod)
			if !ok {
				return
			}

			if !deploy.IsDevWorkspaceComponent(pod.GetLabels()) ||
				pod.Spec.ServiceAccountName != constants.DevWorkspaceServiceAccountName {
				return
			}

			newNamespace := pod.GetNamespace()
			currentNamespace := r.getDWONamespace()

			if currentNamespace == newNamespace {
				return
			}

			r.setDWONamespace(newNamespace)

			for _, namespace := range r.namespaceCache.GetAllKnownNamespaces() {
				q.Add(reconcile.Request{
					NamespacedName: types.NamespacedName{Name: namespace},
				})
			}
		},
		// Handle the delete event to resolve an invalid configuration where
		// multiple DevWorkspace Operators are running in different namespaces.
		DeleteFunc: func(
			ctx context.Context,
			evt event.DeleteEvent,
			q workqueue.TypedRateLimitingInterface[reconcile.Request],
		) {
			pod, ok := evt.Object.(*corev1.Pod)
			if !ok {
				return
			}

			if !deploy.IsDevWorkspaceComponent(pod.GetLabels()) ||
				pod.Spec.ServiceAccountName != constants.DevWorkspaceServiceAccountName {
				return
			}

			currentDWONamespace := r.getDWONamespace()

			// Find the new DWO namespace
			newDWONamespace, err := devworkspace.GetDevWorkspaceOperatorNamespace(ctx, r.clientWrapper)
			if err != nil {
				logger.Error(err, "Failed to get DevWorkspaceOperator namespace")
				return
			}

			if currentDWONamespace == newDWONamespace {
				return
			}

			r.setDWONamespace(newDWONamespace)

			for _, namespace := range r.namespaceCache.GetAllKnownNamespaces() {
				q.Add(reconcile.Request{
					NamespacedName: types.NamespacedName{Name: namespace},
				})
			}
		},
	}
}

func (r *CheUserNamespaceReconciler) setDWONamespace(dwoNamespace string) {
	r.dwoNamespaceMu.Lock()
	defer r.dwoNamespaceMu.Unlock()

	r.dwoNamespace = dwoNamespace
}

func (r *CheUserNamespaceReconciler) getDWONamespace() string {
	r.dwoNamespaceMu.RLock()
	defer r.dwoNamespaceMu.RUnlock()

	return r.dwoNamespace
}
