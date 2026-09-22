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

	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauth "github.com/openshift/api/oauth/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	OAuthFinalizerName = "oauthclients.finalizers.che.eclipse.org"
)

var logger = ctrl.Log.WithName("identity-provider")

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

func (ip *IdentityProviderReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if err := syncOAuthClient(ctx); err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (ip *IdentityProviderReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	oauthClient, err := GetOAuthClient(ctx)
	if err != nil {
		logger.Error(err, "Failed to get OAuthClient")
		return false
	}

	if oauthClient != nil {
		if err := ctx.ClusterAPI.NonCachingClientWrapper.DeleteByKeyIgnoreNotFound(
			ctx.Context,
			types.NamespacedName{Name: oauthClient.Name},
			&oauth.OAuthClient{},
		); err != nil {
			// failed to delete OAuthClient, but it shouldn't prevent us from removing the finalizer
			logger.Error(err, "Failed to delete OAuthClient", "name", oauthClient.Name)
		}
	}

	if err := deploy.DeleteFinalizer(ctx, OAuthFinalizerName); err != nil {
		logger.Error(err, "Failed to delete finalizer", "finalizer", OAuthFinalizerName)
		return false
	}

	return true
}

func syncOAuthClient(ctx *chetypes.DeployContext) error {
	var oauthClientName, oauthSecret string

	oauthClient, err := GetOAuthClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OAuthClient: %w", err)
	}

	if oauthClient != nil {
		oauthClientName = oauthClient.Name
		oauthSecret = utils.GetValue(string(ctx.Authentication.ClientSecret), oauthClient.Secret)
	} else {
		oauthClientName = GetOAuthClientName(ctx)
		oauthSecret = utils.GetValue(string(ctx.Authentication.ClientSecret), utils.GeneratePassword(12))
	}

	redirectURIs := []string{"https://" + ctx.CheHost + "/oauth/callback"}
	oauthClientSpec := GetOAuthClientSpec(
		oauthClientName,
		oauthSecret,
		redirectURIs,
		ctx.CheCluster.Spec.Networking.Auth.OAuthAccessTokenInactivityTimeoutSeconds,
		ctx.CheCluster.Spec.Networking.Auth.OAuthAccessTokenMaxAgeSeconds)

	if err := ctx.ClusterAPI.ClientWrapper.Sync(
		ctx.Context,
		oauthClientSpec,
		&k8sclient.SyncOptions{DiffOpts: oAuthClientDiffOpts, SuppressDiff: true},
	); err != nil {
		return fmt.Errorf("failed to sync OAuthClient %s: %w", oauthClientSpec.Name, err)
	}

	if err := deploy.AppendFinalizer(ctx, OAuthFinalizerName); err != nil {
		return fmt.Errorf("failed to append finalizer %s: %w", OAuthFinalizerName, err)
	}

	return nil
}
