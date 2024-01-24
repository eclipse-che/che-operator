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
	v1SecretGKV = corev1.SchemeGroupVersion.WithKind("Secret")
)

type secretSyncer struct {
	workspaceConfigSyncer
}

func newSecretSyncer() *secretSyncer {
	return &secretSyncer{}
}

func (p *secretSyncer) gkv() schema.GroupVersionKind {
	return v1SecretGKV
}

func (p *secretSyncer) newObjectFrom(src client.Object) client.Object {
	dst := src.(runtime.Object).DeepCopyObject()
	dst.(*corev1.Secret).ObjectMeta = metav1.ObjectMeta{
		Name:        src.GetName(),
		Annotations: src.GetAnnotations(),
		Labels: mergeWorkspaceConfigObjectLabels(
			src.GetLabels(),
			map[string]string{
				dwconstants.DevWorkspaceWatchSecretLabel: "true",
				dwconstants.DevWorkspaceMountLabel:       "true",
			},
		),
	}

	return dst.(client.Object)
}

func (p *secretSyncer) isExistedObjChanged(newObj client.Object, existedObj client.Object) bool {
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
			cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
		}) != ""
}

func (p *secretSyncer) getObjectList() client.ObjectList {
	return &corev1.SecretList{}
}

func (p *secretSyncer) hasReadOnlySpec() bool {
	return false
}
