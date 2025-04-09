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

type configMap2Sync struct {
	Object2Sync

	cm      *corev1.ConfigMap
	version string
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
			{
				dwconstants.DevWorkspaceWatchConfigMapLabel: "true",
				dwconstants.DevWorkspaceMountLabel:          "true",
			},
			p.cm.GetLabels(),
		}),
	}

	return dst.(client.Object)
}

func (p *configMap2Sync) getSrcObjectVersion() string {
	if len(p.version) == 0 {
		return p.cm.GetResourceVersion()
	}
	return p.version
}

func (p *configMap2Sync) hasROSpec() bool {
	return false
}
