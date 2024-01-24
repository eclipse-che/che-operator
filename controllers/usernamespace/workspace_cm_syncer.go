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
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	v1ConfigMapGKV = corev1.SchemeGroupVersion.WithKind("ConfigMap")
)

type configMapSyncer struct {
	workspaceConfigSyncer
}

func newConfigMapSyncer() *configMapSyncer {
	return &configMapSyncer{}
}

func (p *configMapSyncer) gkv() schema.GroupVersionKind {
	return v1ConfigMapGKV
}

func (p *configMapSyncer) newObjectFrom(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.ConfigMap).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels: mergeWorkspaceConfigObjectLabels(
			src.GetLabels(),
			map[string]string{
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
			},
		),
	}

	return dst.(client.Object)
}

func (p *configMapSyncer) isExistedObjChanged(newObj client.Object, existedObj client.Object) bool {
	if newObj.GetLabels() != nil {
		for key, value := range newObj.GetLabels() {
			if existedObj.GetLabels()[key] != value {
				return true
			}
		}
	}

	if newObj.GetAnnotations() != nil {
		for key, value := range newObj.GetAnnotations() {
			if existedObj.GetAnnotations()[key] != value {
				return true
			}
		}
	}

	return cmp.Diff(
		newObj,
		existedObj,
		cmp.Options{
			cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta", "ObjectMeta"),
		}) != ""
}

func (p *configMapSyncer) getObjectList() client.ObjectList {
	return &corev1.ConfigMapList{}
}

func (p *configMapSyncer) hasReadOnlySpec() bool {
	return false
}
