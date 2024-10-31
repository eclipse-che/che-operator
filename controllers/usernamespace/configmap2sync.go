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
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	v1ConfigMapGKV = corev1.SchemeGroupVersion.WithKind("ConfigMap")
)

type configMap2Sync struct {
	Object2Sync
	cm *corev1.ConfigMap
}

func newCM2Sync(cm *corev1.ConfigMap) *configMap2Sync {
	return &configMap2Sync{cm: cm}
}

func (p *configMap2Sync) getSrcObject() client.Object {
	return p.cm
}

func (p *configMap2Sync) getGKV() schema.GroupVersionKind {
	return v1ConfigMapGKV
}

func (p *configMap2Sync) newDstObject() client.Object {
	dst := p.cm.DeepCopyObject()
	// We have to set the ObjectMeta fields explicitly, because
	// existed object contains unnecessary fields that we don't want to copy
	dst.(*corev1.ConfigMap).ObjectMeta = metav1.ObjectMeta{
		Name:        p.cm.GetName(),
		Annotations: p.cm.GetAnnotations(),
		Labels: utils.MergeMaps([]map[string]string{
			p.cm.GetLabels(),
			{
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
			}}),
	}

	return dst.(client.Object)
}

func (p *configMap2Sync) getSrcObjectVersion() string {
	return p.cm.GetResourceVersion()
}

func (p *configMap2Sync) hasROSpec() bool {
	return false
}

func (p *configMap2Sync) isDiff(obj client.Object) bool {
	return isLabelsOrAnnotationsDiff(p.cm, obj) ||
		cmp.Diff(
			p.cm,
			obj,
			cmp.Options{
				cmpopts.IgnoreTypes(metav1.ObjectMeta{}),
				cmpopts.IgnoreTypes(metav1.TypeMeta{}),
			}) != ""
}
