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

package resources

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Job = &jobHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(batchv1.Job{}, "TypeMeta", "Status"),
		cmpopts.IgnoreFields(corev1.Container{}, "TerminationMessagePath", "TerminationMessagePolicy"),
		cmpopts.IgnoreFields(corev1.PodSpec{}, "SchedulerName"),
		cmp.Comparer(func(x, y resource.Quantity) bool { return x.Cmp(y) == 0 }),
	},
}

type jobHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h jobHandler) Sync(cheCtx *chetypes.CheContext, job *batchv1.Job) error {
	return syncher.SyncObject(
		cheCtx,
		job,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h jobHandler) Get(cheCtx *chetypes.CheContext, name string, namespace string, job *batchv1.Job) (bool, error) {
	return syncher.GetObject(
		cheCtx,
		name,
		namespace,
		job,
	)
}

func (h jobHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return syncher.DeleteObject(
		cheCtx,
		name,
		namespace,
		&batchv1.Job{},
		client.PropagationPolicy(metav1.DeletePropagationBackground),
	)
}

func (h jobHandler) IsFinished(job *batchv1.Job) bool {
	return h.GetCompletionCondition(job) != nil
}

func (h jobHandler) IsSucceeded(job *batchv1.Job) bool {
	condition := h.GetCompletionCondition(job)
	return condition != nil && condition.Type == batchv1.JobComplete
}

func (h jobHandler) GetFailureReason(job *batchv1.Job) string {
	condition := h.GetCompletionCondition(job)
	if condition == nil || condition.Type != batchv1.JobFailed {
		return ""
	}

	if condition.Message != "" {
		return condition.Reason + ": " + condition.Message
	}

	return condition.Reason
}

func (h jobHandler) GetCompletionCondition(job *batchv1.Job) *batchv1.JobCondition {
	for i := range job.Status.Conditions {
		condition := job.Status.Conditions[i]
		if condition.Status != corev1.ConditionTrue {
			continue
		}

		if condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed {
			return &job.Status.Conditions[i]
		}
	}

	return nil
}
