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
	consolev1 "github.com/openshift/api/console/v1"
)

var ConsoleLink = &consoleLinkHandler{
	suppressDiff:       false,
	allowForceRecreate: true,
	diff: cmp.Options{
		cmpopts.IgnoreFields(consolev1.ConsoleLink{}, "TypeMeta"),
	},
}

type consoleLinkHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h consoleLinkHandler) Sync(cheCtx *chetypes.CheContext, consoleLink *consolev1.ConsoleLink) error {
	return syncher.SyncObject(
		cheCtx,
		consoleLink,
		h.diff,
		h.suppressDiff,
		h.allowForceRecreate,
	)
}

func (h consoleLinkHandler) Delete(cheCtx *chetypes.CheContext, name string) error {
	return syncher.DeleteObject(
		cheCtx,
		name,
		"",
		&consolev1.ConsoleLink{},
	)
}
