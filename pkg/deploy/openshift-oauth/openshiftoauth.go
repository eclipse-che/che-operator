//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package openshiftoauth

import (
	"context"
	"reflect"
	"strconv"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	warningNoIdentityProvidersMessage = "No Openshift identity providers."

	AddIdentityProviderMessage      = "Openshift oAuth was disabled. How to add identity provider read in the Help Link:"
	warningNoRealUsersMessage       = "No real users. Openshift oAuth was disabled. How to add new user read in the Help Link:"
	failedUnableToGetOpenshiftUsers = "Unable to get users on the OpenShift cluster."

	howToAddIdentityProviderLinkOS4 = "https://docs.openshift.com/container-platform/latest/authentication/understanding-identity-provider.html#identity-provider-overview_understanding-identity-provider"
	howToConfigureOAuthLinkOS3      = "https://docs.openshift.com/container-platform/3.11/install_config/configuring_authentication.html"
)

type OpenShiftOAuth struct {
}

func NewOpenShiftOAuth() *OpenShiftOAuth {
	return &OpenShiftOAuth{}
}

func (oo *OpenShiftOAuth) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if util.IsOpenShift && ctx.CheCluster.Spec.Auth.OpenShiftoAuth == nil {
		return oo.enableOpenShiftOAuth(ctx)
	}

	return reconcile.Result{}, true, nil
}

func (oo *OpenShiftOAuth) Finalize(ctx *deploy.DeployContext) bool {
	return true
}

func (oo *OpenShiftOAuth) enableOpenShiftOAuth(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	oauth := false
	if util.IsOpenShift4 {
		openshitOAuth, err := GetOpenshiftOAuth(ctx)
		if err != nil {
			logrus.Error("Unable to get Openshift oAuth. Cause: " + err.Error())
		} else {
			if len(openshitOAuth.Spec.IdentityProviders) > 0 {
				oauth = true
			} else if ctx.CheCluster.IsNativeUserModeEnabled() {
				// enable OpenShift OAuth without adding initial OpenShift OAuth user
				// since kubeadmin is a valid user for native user mode
				oauth = true
			}
		}
	} else { // Openshift 3
		users := &userv1.UserList{}
		listOptions := &client.ListOptions{}
		if err := ctx.ClusterAPI.NonCachingClient.List(context.TODO(), users, listOptions); err != nil {
			logrus.Error(failedUnableToGetOpenshiftUsers + " Cause: " + err.Error())
		} else {
			oauth = len(users.Items) >= 1
			if !oauth {
				logrus.Warn(warningNoRealUsersMessage + " " + howToConfigureOAuthLinkOS3)
			}
		}
	}

	newOAuthValue := util.NewBoolPointer(oauth)
	if !reflect.DeepEqual(newOAuthValue, ctx.CheCluster.Spec.Auth.OpenShiftoAuth) {
		ctx.CheCluster.Spec.Auth.OpenShiftoAuth = newOAuthValue
		if err := deploy.UpdateCheCRSpec(ctx, "openShiftoAuth", strconv.FormatBool(oauth)); err != nil {
			return reconcile.Result{Requeue: true}, false, err
		}
	}

	return reconcile.Result{}, true, nil
}
