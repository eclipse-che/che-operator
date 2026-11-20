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
	oauthv1 "github.com/openshift/api/oauth/v1"
)

var OAuthClient = &oauthClientHandler{
	suppressDiff:       true,
	allowForceRecreate: false,
	diff: cmp.Options{
		cmpopts.IgnoreFields(oauthv1.OAuthClient{}, "TypeMeta"),
	},
}

type oauthClientHandler struct {
	suppressDiff       bool
	allowForceRecreate bool
	diff               cmp.Options
}

func (h oauthClientHandler) Create(cheCtx *chetypes.CheContext, oauthClient *oauthv1.OAuthClient) error {
	return syncher.Create(cheCtx, oauthClient)
}

func (h oauthClientHandler) Delete(cheCtx *chetypes.CheContext, name string) error {
	return syncher.DeleteObject(
		cheCtx,
		name,
		"",
		&oauthv1.OAuthClient{},
	)
}
