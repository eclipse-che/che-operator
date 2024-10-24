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
	v1SecretGKV = corev1.SchemeGroupVersion.WithKind("Secret")
)

type secretWorkspaceSyncObject struct {
	WorkspaceSyncObject
	secret *corev1.Secret
}

func newSecretWorkspaceSyncObject(secret *corev1.Secret) *secretWorkspaceSyncObject {
	return &secretWorkspaceSyncObject{
		secret: secret,
	}
}

func (p *secretWorkspaceSyncObject) getSrcObjectGKV() schema.GroupVersionKind {
	return v1SecretGKV
}

func (p *secretWorkspaceSyncObject) getSrcObject() client.Object {
	return p.secret
}

func (p *secretWorkspaceSyncObject) newDstObject() client.Object {
	dst := p.secret.DeepCopyObject()
	dst.(*corev1.Secret).ObjectMeta = metav1.ObjectMeta{
		Name:        p.secret.GetName(),
		Annotations: p.secret.GetAnnotations(),
		Labels: utils.MergeMaps([]map[string]string{
			p.secret.GetLabels(),
			{
				dwconstants.DevWorkspaceWatchSecretLabel: "true",
				dwconstants.DevWorkspaceMountLabel:       "true",
			}}),
	}

	return dst.(client.Object)
}

func (p *secretWorkspaceSyncObject) getSrcObjectVersion() string {
	return p.secret.GetResourceVersion()
}

func (p *secretWorkspaceSyncObject) hasROSpec() bool {
	return false
}
