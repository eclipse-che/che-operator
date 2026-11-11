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
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestDevWorkspaceOperatorPodPredicates(t *testing.T) {
	predicates := devWorkspaceOperatorPodPredicates
	dwoPod := test.NewDWOPod("devworkspace-controller", "project-clone:dwo")
	webhookServerPod := newDevWorkspaceOperatorWebhookServerPod()

	t.Run("Create, delete and generic events", func(t *testing.T) {
		assert.True(t, predicates.Create(event.CreateEvent{Object: dwoPod}))
		assert.True(t, predicates.Delete(event.DeleteEvent{Object: dwoPod}))
		assert.True(t, predicates.Generic(event.GenericEvent{Object: dwoPod}))

		assert.False(t, predicates.Create(event.CreateEvent{Object: webhookServerPod}))
		assert.False(t, predicates.Delete(event.DeleteEvent{Object: webhookServerPod}))
		assert.False(t, predicates.Generic(event.GenericEvent{Object: webhookServerPod}))
	})

	t.Run("Update events", func(t *testing.T) {
		statusChanged := dwoPod.DeepCopy()
		statusChanged.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}

		phaseChanged := dwoPod.DeepCopy()
		phaseChanged.Status.Phase = corev1.PodFailed

		terminating := dwoPod.DeepCopy()
		terminating.DeletionTimestamp = ptr.To(metav1.Now())

		webhookServerPodPhaseChanged := webhookServerPod.DeepCopy()
		webhookServerPodPhaseChanged.Status.Phase = corev1.PodFailed

		assert.False(t, predicates.Update(event.UpdateEvent{ObjectOld: dwoPod, ObjectNew: statusChanged}))
		assert.True(t, predicates.Update(event.UpdateEvent{ObjectOld: dwoPod, ObjectNew: phaseChanged}))
		assert.True(t, predicates.Update(event.UpdateEvent{ObjectOld: dwoPod, ObjectNew: terminating}))
		assert.False(t, predicates.Update(
			event.UpdateEvent{ObjectOld: webhookServerPod, ObjectNew: webhookServerPodPhaseChanged},
		))
	})
}

func TestInNamespaceEventFilter(t *testing.T) {
	filter := inNamespaceEventFilter("eclipse-che")

	inWatchedNamespace := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "eclipse-che"}}
	inAnotherNamespace := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"}}
	dwoPod := test.NewDWOPod("devworkspace-controller", "project-clone:dwo")
	webhookServerPod := newDevWorkspaceOperatorWebhookServerPod()

	assert.True(t, filter.Create(event.CreateEvent{Object: inWatchedNamespace}))
	assert.False(t, filter.Create(event.CreateEvent{Object: inAnotherNamespace}))
	assert.True(t, filter.Create(event.CreateEvent{Object: dwoPod}))
	assert.False(t, filter.Create(event.CreateEvent{Object: webhookServerPod}))
	assert.True(t, filter.Update(event.UpdateEvent{ObjectOld: dwoPod, ObjectNew: dwoPod}))
	assert.True(t, filter.Delete(event.DeleteEvent{Object: dwoPod}))
}
