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
	"maps"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
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
	dstLabels := map[string]string{
		dwconstants.DevWorkspaceWatchSecretLabel: "true",
		dwconstants.DevWorkspaceMountLabel:       "true",
	}
	maps.Copy(dstLabels, p.secret.GetLabels())
	dst.(*corev1.Secret).ObjectMeta = metav1.ObjectMeta{
		Name:        p.secret.GetName(),
		Annotations: p.secret.GetAnnotations(),
		Labels:      dstLabels,
	}

	return dst.(client.Object)
}

func (p *secret2Sync) getSrcObjectVersion() string {
	if p.version != "" {
		return p.version
	}

	return p.secret.GetResourceVersion()
}

func (p *secret2Sync) hasROSpec() bool {
	return false
}

func (p *secret2Sync) defaultRetention() bool {
	return false
}
