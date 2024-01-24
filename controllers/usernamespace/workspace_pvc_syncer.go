//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

type pvcSyncer struct {
	workspaceConfigSyncer
}

func newPvcSyncer() *pvcSyncer {
	return &pvcSyncer{}
}

func (p *pvcSyncer) gkv() schema.GroupVersionKind {
	return v1PvcGKV
}

func (p *pvcSyncer) newObjectFrom(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.PersistentVolumeClaim).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels:      src.GetLabels(),
	}
	dst.(*corev1.PersistentVolumeClaim).Status = corev1.PersistentVolumeClaimStatus{}

	return dst.(client.Object)
}

func (p *pvcSyncer) isExistedObjChanged(newObj client.Object, existedObj client.Object) bool {
	return false
}

func (p *pvcSyncer) getObjectList() client.ObjectList {
	return &corev1.PersistentVolumeClaimList{}
}

func (p *pvcSyncer) hasReadOnlySpec() bool {
	return true
}
