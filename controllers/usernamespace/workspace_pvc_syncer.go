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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	v1PvcGKV = corev1.SchemeGroupVersion.WithKind("PersistentVolumeClaim")
)

type pvcWorkspaceSyncObject struct {
	WorkspaceSyncObject
	pvc *corev1.PersistentVolumeClaim
}

func newPvcWorkspaceSyncObject(pvc *corev1.PersistentVolumeClaim) *pvcWorkspaceSyncObject {
	return &pvcWorkspaceSyncObject{
		pvc: pvc,
	}
}

func (p *pvcWorkspaceSyncObject) getGKV() schema.GroupVersionKind {
	return v1PvcGKV
}

func (p *pvcWorkspaceSyncObject) newDstObj(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.PersistentVolumeClaim).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels:      src.GetLabels(),
	}
	dst.(*corev1.PersistentVolumeClaim).Status = corev1.PersistentVolumeClaimStatus{}

	return dst.(client.Object)
}

func (p *pvcWorkspaceSyncObject) getSrcObject() client.Object {
	return p.pvc
}

func (p *pvcWorkspaceSyncObject) newDstObject() client.Object {
	dst := p.pvc.DeepCopyObject()
	dst.(*corev1.PersistentVolumeClaim).ObjectMeta = metav1.ObjectMeta{
		Name:        p.pvc.GetName(),
		Annotations: p.pvc.GetAnnotations(),
		Labels:      p.pvc.GetLabels(),
	}
	dst.(*corev1.PersistentVolumeClaim).Status = corev1.PersistentVolumeClaimStatus{}

	return dst.(client.Object)
}

func (p *pvcWorkspaceSyncObject) getSrcObjectVersion() string {
	return p.pvc.GetResourceVersion()
}

func (p *pvcWorkspaceSyncObject) hasROSpec() bool {
	return true
}
