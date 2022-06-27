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
	"strings"

	"github.com/eclipse-che/che-operator/pkg/common/utils"
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
	oauthClients, err := FindAllEclipseCheOAuthClients(ctx)
	if err != nil {
		logrus.Errorf("Error getting OAuthClients: %v", err)
		return false
	}

	for _, oauthClient := range oauthClients {
		if _, err := deploy.DeleteClusterObject(ctx, oauthClient.Name, &oauth.OAuthClient{}); err != nil {
			logrus.Errorf("Error deleting OAuthClient: %v", err)
			return false
		}
	}

	if err := deploy.DeleteFinalizer(ctx, OAuthFinalizerName); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}

	return true
}

func syncOAuthClient(ctx *chetypes.DeployContext) (bool, error) {
	oauthClientName := ctx.CheCluster.Spec.Networking.Auth.OAuthClientName
	oauthSecret := ctx.CheCluster.Spec.Networking.Auth.OAuthSecret

	if oauthClientName == "" {
		oauthClient, err := FindOAuthClient(ctx)
		if err != nil {
			logrus.Errorf("Error getting OAuthClients: %v", err)
			return false, err
		}

		if oauthClient != nil {
			oauthClientName = oauthClient.Name
			if oauthSecret == "" {
				oauthSecret = oauthClient.Secret
			}
		}
	} else {
		oauthClient := &oauth.OAuthClient{}
		exists, _ := deploy.GetClusterObject(ctx, oauthClientName, oauthClient)
		if exists {
			if oauthSecret == "" {
				oauthSecret = oauthClient.Secret
			}
		}
	}

	// Generate secret and name
	oauthSecret = utils.GetValue(oauthSecret, utils.GeneratePassword(12))
	oauthClientName = utils.GetValue(oauthClientName, ctx.CheCluster.Name+"-openshift-identity-provider-"+strings.ToLower(utils.GeneratePassword(6)))

	redirectURIs := []string{"https://" + ctx.CheHost + "/oauth/callback"}
	oauthClientSpec := GetOAuthClientSpec(oauthClientName, oauthSecret, redirectURIs)
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
