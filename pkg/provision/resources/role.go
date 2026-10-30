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
	rbac "k8s.io/api/rbac/v1"
)

var Role = &roleHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(rbac.Role{}, "TypeMeta"),
	},
}

type roleHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h roleHandler) Sync(cheCtx *chetypes.CheContext, role *rbac.Role) error {
	return syncher.SyncObject(
		cheCtx,
		role,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h roleHandler) Delete(cheCtx *chetypes.CheContext, name string, namespace string) error {
	return syncher.DeleteObject(
		cheCtx,
		name,
		namespace,
		&rbac.Role{},
	)
}
