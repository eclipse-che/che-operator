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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	v1ConfigMapGKV = corev1.SchemeGroupVersion.WithKind("ConfigMap")
)

type cmWorkspaceSyncObject struct {
	WorkspaceSyncObject
	cm *corev1.ConfigMap
}

func newCMWorkspaceSyncObject(cm *corev1.ConfigMap) *cmWorkspaceSyncObject {
	return &cmWorkspaceSyncObject{cm: cm}
}

func (p *cmWorkspaceSyncObject) getSrcObject() client.Object {
	return p.cm
}

func (p *cmWorkspaceSyncObject) getSrcObjectGKV() schema.GroupVersionKind {
	return v1ConfigMapGKV
}

func (p *cmWorkspaceSyncObject) newDstObject() client.Object {
	dst := p.cm.DeepCopyObject()
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

func (p *cmWorkspaceSyncObject) getSrcObjectVersion() string {
	return p.cm.GetResourceVersion()
}

func (p *cmWorkspaceSyncObject) hasROSpec() bool {
	return false
}
