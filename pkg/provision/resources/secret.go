//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package resources

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/provision/syncher"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
)

var Secret = &secretHandler{
	suppressDiff:       true,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta"),
	},
}

type secretHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h secretHandler) Sync(cheCtx *chetypes.CheContext, secret *corev1.Secret) error {
	return syncher.SyncObject(
		cheCtx,
		secret,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h secretHandler) Get(cheCtx *chetypes.CheContext, name string, namespace string, secret *corev1.Secret) (bool, error) {
	return syncher.GetObject(
		cheCtx,
		name,
		namespace,
		secret,
	)
}

func (h secretHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return syncher.DeleteObject(
		cheCtx,
		name,
		namespace,
		&corev1.Secret{},
	)
}
