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

package provision

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	networkingv1 "k8s.io/api/networking/v1"
)

var NetworkPolicy = &networkPolicyHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(networkingv1.NetworkPolicy{}, "TypeMeta"),
	},
}

type networkPolicyHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h networkPolicyHandler) Sync(cheCtx *chetypes.CheContext, policy *networkingv1.NetworkPolicy) error {
	return syncObject(
		cheCtx,
		policy,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h networkPolicyHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return deleteObject(
		cheCtx,
		name,
		namespace,
		&networkingv1.NetworkPolicy{},
	)
}

func (h networkPolicyHandler) DeleteAll(cheCtx *chetypes.CheContext, namespace string) error {
	return deleteAllManaged(
		cheCtx,
		namespace,
		&networkingv1.NetworkPolicy{},
		defaults.GetCheFlavor(),
	)
}
