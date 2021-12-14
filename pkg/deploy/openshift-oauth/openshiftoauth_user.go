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

package openshiftoauth

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	HtpasswdIdentityProviderName        = "htpasswd-eclipse-che"
	HtpasswdSecretName                  = "htpasswd-eclipse-che"
	OcConfigNamespace                   = "openshift-config"
	OpenShiftOAuthUserCredentialsSecret = "openshift-oauth-user-credentials"
	OpenshiftOauthUserFinalizerName     = "openshift-oauth-user.finalizers.che.eclipse.org"
)

type IOpenShiftOAuthUser interface {
	Create(ctx *deploy.DeployContext) (bool, error)
	Delete(ctx *deploy.DeployContext) error
}

type OpenShiftOAuthUser struct {
	userPassword string
	runnable     util.Runnable

	deploy.Reconcilable
	IOpenShiftOAuthUser
}

func NewOpenShiftOAuthUser() *OpenShiftOAuthUser {
	return &OpenShiftOAuthUser{
		userPassword: util.GeneratePasswd(6),
		runnable:     util.NewRunnable(), // real process, mock for tests
	}
}

func (oou *OpenShiftOAuthUser) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if !util.IsOpenShift4 {
		return reconcile.Result{}, true, nil
	}

	if ctx.CheCluster.IsOpenShiftOAuthUserConfigured() &&
		(ctx.CheCluster.Spec.Auth.OpenShiftoAuth == nil || *ctx.CheCluster.Spec.Auth.OpenShiftoAuth) {

		done, err := oou.Create(ctx)
		if !done {
			return reconcile.Result{Requeue: true}, false, err
		}
		return reconcile.Result{}, true, nil
	}

	if ctx.CheCluster.IsOpenShiftOAuthUserMustBeDeleted() {
		if done := oou.Delete(ctx); !done {
			return reconcile.Result{}, false, fmt.Errorf("unable to delete initial OpenShift OAuth user from a cluster")
		}

		return reconcile.Result{}, true, nil
	}

	return reconcile.Result{}, true, nil
}

func (oou *OpenShiftOAuthUser) Finalize(ctx *deploy.DeployContext) bool {
	if util.IsOpenShift4 {
		return oou.Delete(ctx)
	}

	return true
}

// Creates new htpasswd provider with initial user with Che flavor name
// if Openshift cluster hasn't got identity providers then does nothing.
// It usefull for good first user experience.
// User can't use kube:admin or system:admin user in the Openshift oAuth when DevWorkspace engine disabled.
// That's why we provide initial user for good first meeting with Eclipse Che.
func (oou *OpenShiftOAuthUser) Create(ctx *deploy.DeployContext) (bool, error) {
	userName := deploy.DefaultCheFlavor(ctx.CheCluster)

	// read existed password from the secret (operator has been restarted case)
	oAuthUserCredentialsSecret, err := oou.getOpenShiftOAuthUserCredentialsSecret(ctx)
	if err != nil {
		return false, err
	} else if oAuthUserCredentialsSecret != nil {
		oou.userPassword = string(oAuthUserCredentialsSecret.Data["password"])
	}

	// create a new secret with user's credentials
	if oAuthUserCredentialsSecret == nil {
		initialUserSecretData := map[string][]byte{"user": []byte(userName), "password": []byte(oou.userPassword)}
		done, err := deploy.SyncSecretToCluster(ctx, OpenShiftOAuthUserCredentialsSecret, OcConfigNamespace, initialUserSecretData)
		if !done {
			return false, err
		}
	}

	htpasswdSecretExists, _ := deploy.Get(ctx, types.NamespacedName{Name: HtpasswdSecretName, Namespace: OcConfigNamespace}, &corev1.Secret{})
	if !htpasswdSecretExists {
		htpasswdFileContent, err := oou.generateHtPasswdUserInfo(userName, oou.userPassword)
		if err != nil {
			return false, err
		}

		htpasswdFileSecretData := map[string][]byte{"htpasswd": []byte(htpasswdFileContent)}
		done, err := deploy.SyncSecretToCluster(ctx, HtpasswdSecretName, OcConfigNamespace, htpasswdFileSecretData)
		if !done {
			return false, err
		}
	}

	oAuth, err := GetOpenshiftOAuth(ctx)
	if err != nil {
		return false, err
	}

	if err := appendIdentityProvider(oAuth, ctx.ClusterAPI.NonCachingClient); err != nil {
		return false, err
	}

	if ctx.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret != OpenShiftOAuthUserCredentialsSecret {
		ctx.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret = OpenShiftOAuthUserCredentialsSecret
		if err := deploy.UpdateCheCRStatus(ctx, "openShiftOAuthUserCredentialsSecret", OpenShiftOAuthUserCredentialsSecret); err != nil {
			return false, err
		}
	}

	if err := deploy.AppendFinalizer(ctx, OpenshiftOauthUserFinalizerName); err != nil {
		return false, err
	}

	return true, nil
}

// Removes initial user, htpasswd provider, htpasswd secret and Che secret with username and password.
func (oou *OpenShiftOAuthUser) Delete(ctx *deploy.DeployContext) bool {
	oAuth, err := GetOpenshiftOAuth(ctx)
	if err != nil {
		logrus.Errorf("Failed to get Openshift OAuth, cause: %v", err)
	}

	userName := deploy.DefaultCheFlavor(ctx.CheCluster)
	if _, err := deploy.Delete(ctx, types.NamespacedName{Name: userName}, &userv1.User{}); err != nil {
		logrus.Errorf("Failed to delete Openshift user '%s', cause: %v", userName, err)
	}

	identityName := HtpasswdIdentityProviderName + ":" + userName
	if _, err := deploy.Delete(ctx, types.NamespacedName{Name: identityName}, &userv1.Identity{}); err != nil {
		logrus.Errorf("Failed to delete identity '%s', cause: %v", identityName, err)
	}

	if err := deleteIdentityProvider(oAuth, ctx.ClusterAPI.NonCachingClient); err != nil {
		logrus.Errorf("Failed to delete identity provider', cause: %v", err)
	}

	if _, err = deploy.Delete(ctx, types.NamespacedName{Name: HtpasswdSecretName, Namespace: OcConfigNamespace}, &corev1.Secret{}); err != nil {
		logrus.Errorf("Failed to delete HTpasswd secret '%s', cause: %v", HtpasswdSecretName, err)
	}

	// legacy secret in the current namespace
	if _, err = deploy.DeleteNamespacedObject(ctx, OpenShiftOAuthUserCredentialsSecret, &corev1.Secret{}); err != nil {
		logrus.Errorf("Failed to delete legacy Openshift OAuth credentials secret '%s', cause: %v", OpenShiftOAuthUserCredentialsSecret, err)
	}

	if _, err = deploy.Delete(ctx, types.NamespacedName{Name: OpenShiftOAuthUserCredentialsSecret, Namespace: OcConfigNamespace}, &corev1.Secret{}); err != nil {
		logrus.Errorf("Failed to delete Openshift OAuth credentials secret '%s', cause: %v", OpenShiftOAuthUserCredentialsSecret, err)
	}

	if err := deploy.DeleteFinalizer(ctx, OpenshiftOauthUserFinalizerName); err != nil {
		logrus.Errorf("Error deleting finalizer: %v", err)
		return false
	}

	ctx.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret = ""
	if err := deploy.UpdateCheCRStatus(ctx, "openShiftOAuthUserCredentialsSecret", ""); err != nil {
		logrus.Errorf("Filed to update openShiftOAuthUserCredentialsSecret in CR status, cause: %v", err)
		return false
	}

	// set 'openShiftoAuth:nil` to reenable on the following reconcile loop (if possible)
	ctx.CheCluster.Spec.Auth.InitialOpenShiftOAuthUser = nil
	ctx.CheCluster.Spec.Auth.OpenShiftoAuth = nil
	updateFields := map[string]string{
		"openShiftoAuth":            "nil",
		"initialOpenShiftOAuthUser": "nil",
	}

	if err := deploy.UpdateCheCRSpecByFields(ctx, updateFields); err != nil {
		logrus.Errorf("Filed to update OAuth field in CR, cause: %v", err)
		return false
	}

	return true
}

func (oou *OpenShiftOAuthUser) generateHtPasswdUserInfo(userName string, password string) (string, error) {
	if util.IsTestMode() {
		return "", nil
	}

	err := oou.runnable.Run("htpasswd", "-nbB", userName, password)
	if err != nil {
		return "", err
	}

	if len(oou.runnable.GetStdErr()) > 0 {
		return "", fmt.Errorf("Failed to generate data for HTPasswd identity provider: %s", oou.runnable.GetStdErr())
	}
	return oou.runnable.GetStdOut(), nil
}

// Gets OpenShift user credentials secret from from the secret from:
// - openshift-config namespace
// - eclipse-che namespace
func (oou *OpenShiftOAuthUser) getOpenShiftOAuthUserCredentialsSecret(ctx *deploy.DeployContext) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	exists, err := deploy.Get(ctx, types.NamespacedName{Name: OpenShiftOAuthUserCredentialsSecret, Namespace: OcConfigNamespace}, secret)
	if err != nil {
		return nil, err
	} else if exists {
		return secret, nil
	}

	exists, err = deploy.GetNamespacedObject(ctx, OpenShiftOAuthUserCredentialsSecret, secret)
	if err != nil {
		return nil, err
	} else if exists {
		return secret, nil
	}

	return nil, nil
}

func identityProviderExists(providerName string, oAuth *configv1.OAuth) bool {
	if len(oAuth.Spec.IdentityProviders) == 0 {
		return false
	}
	for _, identityProvider := range oAuth.Spec.IdentityProviders {
		if identityProvider.Name == providerName {
			return true
		}
	}
	return false
}

func appendIdentityProvider(oAuth *configv1.OAuth, runtimeClient client.Client) error {
	htpasswdProvider := newHtpasswdProvider()
	if !identityProviderExists(htpasswdProvider.Name, oAuth) {
		oauthPatch := client.MergeFrom(oAuth.DeepCopy())

		oAuth.Spec.IdentityProviders = append(oAuth.Spec.IdentityProviders, *htpasswdProvider)

		if err := runtimeClient.Patch(context.TODO(), oAuth, oauthPatch); err != nil {
			return err
		}
	}

	return nil
}

func newHtpasswdProvider() *configv1.IdentityProvider {
	return &configv1.IdentityProvider{
		Name:          HtpasswdIdentityProviderName,
		MappingMethod: configv1.MappingMethodClaim,
		IdentityProviderConfig: configv1.IdentityProviderConfig{
			Type: "HTPasswd",
			HTPasswd: &configv1.HTPasswdIdentityProvider{
				FileData: configv1.SecretNameReference{Name: HtpasswdSecretName},
			},
		},
	}
}

func deleteIdentityProvider(oAuth *configv1.OAuth, runtimeClient client.Client) error {
	logrus.Info("Delete initial user httpasswd provider from the oAuth")

	oauthPatch := client.MergeFrom(oAuth.DeepCopy())
	ips := oAuth.Spec.IdentityProviders
	for i, ip := range ips {
		if ip.Name == HtpasswdIdentityProviderName {
			// remove provider from slice
			oAuth.Spec.IdentityProviders = append(ips[:i], ips[i+1:]...)
			break
		}
	}

	if err := runtimeClient.Patch(context.TODO(), oAuth, oauthPatch); err != nil {
		return err
	}

	return nil
}
