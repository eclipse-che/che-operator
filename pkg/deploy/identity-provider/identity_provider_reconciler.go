//
// Copyright (c) 2019-2021 Red Hat, Inc.
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
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
)

const (
	OAuthFinalizerName = "oauthclients.finalizers.che.eclipse.org"
)

var (
	oAuthClientDiffOpts = cmpopts.IgnoreFields(oauth.OAuthClient{}, "TypeMeta", "ObjectMeta")
)

type IdentityProviderReconciler struct {
	deploy.Reconcilable
}

func NewIdentityProviderReconciler() *IdentityProviderReconciler {
	return &IdentityProviderReconciler{}
}

func (ip *IdentityProviderReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	done, err := syncOAuthClient(ctx)
	return reconcile.Result{Requeue: !done}, done, err
}

func (ip *IdentityProviderReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	oauthClient, err := GetOAuthClient(ctx)
	if err != nil {
		logrus.Errorf("Error getting OAuthClients: %v", err)
		return false
	}

	if oauthClient != nil {
		if err := deploy.DeleteObjectWithFinalizer(ctx, types.NamespacedName{Name: oauthClient.Name}, &oauth.OAuthClient{}, OAuthFinalizerName); err != nil {
			logrus.Errorf("Error deleting OAuthClient: %v", err)
			return false
		}
	}

	return true
}

func syncOAuthClient(ctx *chetypes.DeployContext) (bool, error) {
	var oauthClientName, oauthSecret string

	oauthClient, err := GetOAuthClient(ctx)
	if err != nil {
		logrus.Errorf("Error getting OAuthClients: %v", err)
		return false, err
	}

	if oauthClient != nil {
		oauthClientName = oauthClient.Name
		oauthSecret = utils.GetValue(ctx.CheCluster.Spec.Networking.Auth.OAuthSecret, oauthClient.Secret)
	} else {
		oauthClientName = GetOAuthClientName(ctx)
		oauthSecret = utils.GetValue(ctx.CheCluster.Spec.Networking.Auth.OAuthSecret, utils.GeneratePassword(12))
	}

	redirectURIs := []string{"https://" + ctx.CheHost + "/oauth/callback"}
	oauthClientSpec := GetOAuthClientSpec(
		oauthClientName,
		oauthSecret,
		redirectURIs,
		ctx.CheCluster.Spec.Networking.Auth.OAuthAccessTokenInactivityTimeoutSeconds,
		ctx.CheCluster.Spec.Networking.Auth.OAuthAccessTokenMaxAgeSeconds)
	done, err := deploy.Sync(ctx, oauthClientSpec, oAuthClientDiffOpts)
	if !done {
		return false, err
	}

	err = deploy.AppendFinalizer(ctx, OAuthFinalizerName)
	if err != nil {
		return false, err
	}

	return true, nil
}
