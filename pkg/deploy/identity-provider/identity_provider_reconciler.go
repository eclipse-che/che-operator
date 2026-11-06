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

package identityprovider

import (
	"fmt"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauth "github.com/openshift/api/oauth/v1"
)

var (
	oAuthClientDiffOpts = cmp.Options{
		cmpopts.IgnoreFields(oauth.OAuthClient{}, "TypeMeta", "ObjectMeta"),
	}
)

type IdentityProviderReconciler struct {
	reconciler.Reconcilable
}

func NewIdentityProviderReconciler() *IdentityProviderReconciler {
	return &IdentityProviderReconciler{}
}

func (ip *IdentityProviderReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := syncOAuthClient(cheCtx); err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (ip *IdentityProviderReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	opts := &client.DeleteAllOfOptions{
		ListOptions: client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{constants.KubernetesPartOfLabelKey: constants.CheEclipseOrg}),
		},
	}
	return cheCtx.ClusterAPI.ClientWrapper.DeleteAllOf(cheCtx.Context, &oauth.OAuthClient{}, opts)
}

func syncOAuthClient(cheCtx *chetypes.CheContext) error {
	var oauthClientName, oauthSecret string

	oauthClient, err := GetOAuthClient(cheCtx)
	if err != nil {
		return fmt.Errorf("failed to get OAuthClient: %w", err)
	}

	if oauthClient != nil {
		oauthClientName = oauthClient.Name
		oauthSecret = utils.ValueOrDef(string(cheCtx.Authentication.ClientSecret), oauthClient.Secret)
	} else {
		oauthClientName = GetOAuthClientName(cheCtx)
		oauthSecret = utils.ValueOrDef(string(cheCtx.Authentication.ClientSecret), utils.RandomString(12))
	}

	redirectURIs := []string{"https://" + cheCtx.CheHost + "/oauth/callback"}
	oauthClientSpec := GetOAuthClientSpec(
		oauthClientName,
		oauthSecret,
		redirectURIs,
		cheCtx.CheCluster.Spec.Networking.Auth.OAuthAccessTokenInactivityTimeoutSeconds,
		cheCtx.CheCluster.Spec.Networking.Auth.OAuthAccessTokenMaxAgeSeconds)

	if err := cheCtx.ClusterAPI.NonCachingClientWrapper.Sync(
		cheCtx.Context,
		oauthClientSpec,
		&clientwrapper.SyncOptions{DiffOpts: oAuthClientDiffOpts, SuppressDiff: true},
	); err != nil {
		return fmt.Errorf("failed to sync OAuthClient %s: %w", oauthClientSpec.Name, err)
	}

	return nil
}
