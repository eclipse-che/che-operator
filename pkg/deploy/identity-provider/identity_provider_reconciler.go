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

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp/cmpopts"
	oauth "github.com/openshift/api/oauth/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
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
	done, err := syncNativeIdentityProviderItems(ctx)
	return reconcile.Result{Requeue: !done}, done, err
}

func (ip *IdentityProviderReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	oauthClient, err := FindOAuthClient(ctx)
	if err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}

	if oauthClient != nil {
		err = deploy.DeleteObjectWithFinalizer(ctx, types.NamespacedName{Name: oauthClient.Name}, &oauth.OAuthClient{}, OAuthFinalizerName)
	} else {
		err = deploy.DeleteFinalizer(ctx, OAuthFinalizerName)
	}

	if err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}
	return true
}

func syncNativeIdentityProviderItems(ctx *chetypes.DeployContext) (bool, error) {
	oauthSecret := utils.GeneratePassword(12)
	oauthClientName := ctx.CheCluster.Name + "-openshift-identity-provider-" + strings.ToLower(utils.GeneratePassword(6))

	oauthClient, err := FindOAuthClient(ctx)
	if err != nil {
		return false, err
	}

	if oauthClient != nil {
		oauthSecret = oauthClient.Secret
		oauthClientName = oauthClient.Name
	}

	redirectURIs := []string{"https://" + ctx.CheCluster.GetCheHost() + "/oauth/callback"}
	oauthClientSpec := getOAuthClientSpec(oauthClientName, oauthSecret, redirectURIs)
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
