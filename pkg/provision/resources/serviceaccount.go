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

var ServiceAccount = &serviceAccountHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		// `Secrets` and `ImagePullSecrets` are managed by the cluster: the token controller
		// injects token references, and OpenShift injects image pull secrets. They must not
		// be part of the diff, otherwise every reconcile would fight the cluster over them.
		cmpopts.IgnoreFields(corev1.ServiceAccount{}, "TypeMeta", "Secrets", "ImagePullSecrets", "AutomountServiceAccountToken"),
	},
}

type serviceAccountHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h serviceAccountHandler) Sync(cheCtx *chetypes.CheContext, serviceAccount *corev1.ServiceAccount) error {
	return syncher.SyncObject(
		cheCtx,
		serviceAccount,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h serviceAccountHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return syncher.DeleteObject(
		cheCtx,
		name,
		namespace,
		&corev1.ServiceAccount{},
	)
}
