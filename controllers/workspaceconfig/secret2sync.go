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

package workspace_config

import (
	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type secret2Sync struct {
	Object2Sync

	secret  *corev1.Secret
	version string
}

func (p *secret2Sync) getGKV() schema.GroupVersionKind {
	return v1SecretGKV
}

func (p *secret2Sync) getSrcObject() client.Object {
	return p.secret
}

func (p *secret2Sync) newDstObject() client.Object {
	dst := p.secret.DeepCopyObject()
	// We have to set the ObjectMeta fields explicitly, because
	// existed object contains unnecessary fields that we don't want to copy
	dst.(*corev1.Secret).ObjectMeta = metav1.ObjectMeta{
		Name:        p.secret.GetName(),
		Annotations: p.secret.GetAnnotations(),
		Labels: utils.MergeMaps([]map[string]string{
			{
				dwconstants.DevWorkspaceWatchSecretLabel: "true",
				dwconstants.DevWorkspaceMountLabel:       "true",
			},
			p.secret.GetLabels(),
		}),
	}

	return dst.(client.Object)
}

func (p *secret2Sync) getSrcObjectVersion() string {
	if len(p.version) == 0 {
		return p.secret.GetResourceVersion()
	}
	return p.version
}

func (p *secret2Sync) hasROSpec() bool {
	return false
}
