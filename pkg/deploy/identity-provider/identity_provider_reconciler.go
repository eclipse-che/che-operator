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

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
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

func (ip *IdentityProviderReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	done, err := syncNativeIdentityProviderItems(ctx)
	if !done {
		return reconcile.Result{Requeue: true}, false, err
	}
	return reconcile.Result{}, true, nil
}

func (ip *IdentityProviderReconciler) Finalize(ctx *deploy.DeployContext) bool {
	var err error

	oAuthClientName := ctx.CheCluster.Spec.Auth.OAuthClientName
	if oAuthClientName != "" {
		err = deploy.DeleteObjectWithFinalizer(ctx, types.NamespacedName{Name: oAuthClientName}, &oauth.OAuthClient{}, OAuthFinalizerName)
	} else {
		err = deploy.DeleteFinalizer(ctx, OAuthFinalizerName)
	}

	if err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}
	return true
}

func syncNativeIdentityProviderItems(deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster

	if err := resolveOpenshiftOAuthClientName(deployContext); err != nil {
		return false, err
	}
	if err := resolveOpenshiftOAuthClientSecret(deployContext); err != nil {
		return false, err
	}

	if util.IsOpenShift {
		redirectURIs := []string{"https://" + deployContext.CheHost + "/oauth/callback"}
		oAuthClient := getOAuthClientSpec(cr.Spec.Auth.OAuthClientName, cr.Spec.Auth.OAuthSecret, redirectURIs)
		done, err := deploy.Sync(deployContext, oAuthClient, oAuthClientDiffOpts)
		if !done {
			return false, err
		}

		err = deploy.AppendFinalizer(deployContext, OAuthFinalizerName)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func resolveOpenshiftOAuthClientName(deployContext *deploy.DeployContext) error {
	cr := deployContext.CheCluster
	oAuthClientName := cr.Spec.Auth.OAuthClientName
	if len(oAuthClientName) < 1 {
		oAuthClientName = cr.Name + "-openshift-identity-provider-" + strings.ToLower(util.GeneratePasswd(6))
		cr.Spec.Auth.OAuthClientName = oAuthClientName
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuthClient name", oAuthClientName); err != nil {
			return err
		}
	}
	return nil
}

func resolveOpenshiftOAuthClientSecret(deployContext *deploy.DeployContext) error {
	cr := deployContext.CheCluster
	oauthSecret := cr.Spec.Auth.OAuthSecret
	if len(oauthSecret) < 1 {
		oauthSecret = util.GeneratePasswd(12)
		cr.Spec.Auth.OAuthSecret = oauthSecret
		if err := deploy.UpdateCheCRSpec(deployContext, "oAuth secret name", oauthSecret); err != nil {
			return err
		}
	}
	return nil
}
