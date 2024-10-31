//
// Copyright (c) 2019-2024 Red Hat, Inc.
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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	v1PvcGKV = corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
)

type pvc2Sync struct {
	Object2Sync
	pvc *corev1.PersistentVolumeClaim
}

func newPvc2Sync(pvc *corev1.PersistentVolumeClaim) *pvc2Sync {
	return &pvc2Sync{
		pvc: pvc,
	}
}

func (p *pvc2Sync) getGKV() schema.GroupVersionKind {
	return v1PvcGKV
}

func (p *pvc2Sync) getSrcObject() client.Object {
	return p.pvc
}

func (p *pvc2Sync) newDstObject() client.Object {
	dst := p.pvc.DeepCopyObject()
	// We have to set the ObjectMeta fields explicitly, because
	// existed object contains unnecessary fields that we don't want to copy
	dst.(*corev1.PersistentVolumeClaim).ObjectMeta = metav1.ObjectMeta{
		Name:        p.pvc.GetName(),
		Annotations: p.pvc.GetAnnotations(),
		Labels:      p.pvc.GetLabels(),
	}
	dst.(*corev1.PersistentVolumeClaim).Status = corev1.PersistentVolumeClaimStatus{}

	return dst.(client.Object)
}

func (p *pvc2Sync) getSrcObjectVersion() string {
	return p.pvc.GetResourceVersion()
}

func (p *pvc2Sync) hasROSpec() bool {
	return true
}

func (p *pvc2Sync) isDiff(obj client.Object) bool {
	return isLabelsOrAnnotationsDiff(p.pvc, obj) ||
		cmp.Diff(
			p.pvc,
			obj,
			cmp.Options{
				cmpopts.IgnoreTypes(metav1.ObjectMeta{}),
				cmpopts.IgnoreTypes(metav1.TypeMeta{}),
				cmpopts.IgnoreTypes(corev1.PersistentVolumeClaimStatus{}),
			}) != ""
}
