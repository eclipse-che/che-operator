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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/pkg/errors"
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

var (
	password            = util.GeneratePasswd(6)
	htpasswdFileContent string
)

type IOpenShiftOAuthUser interface {
	Create(ctx *deploy.DeployContext) (bool, error)
	Delete(ctx *deploy.DeployContext) error
}

type OpenShiftOAuthUser struct {
	runnable util.Runnable
}

func NewOpenShiftOAuthUser() *OpenShiftOAuthUser {
	return &OpenShiftOAuthUser{
		// real process, mock for tests
		runnable: util.NewRunnable(),
	}
}

func (oou *OpenShiftOAuthUser) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if !util.IsOpenShift4 {
		return reconcile.Result{}, true, nil
	}

	if ctx.CheCluster.IsOpenShiftOAuthUserConfigured() {
		done, err := oou.Create(ctx)
		if !done {
			return reconcile.Result{Requeue: true}, false, err
		}
		return reconcile.Result{}, true, nil
	}

	if ctx.CheCluster.IsOpenShiftOAuthUserMustBeDeleted() {
		if err := oou.Delete(ctx); err != nil {
			return reconcile.Result{}, false, errors.Wrap(err, "Unable to delete initial OpenShift OAuth user from a cluster")
		}

		ctx.CheCluster.Status.OpenShiftOAuthUserCredentialsSecret = ""
		if err := deploy.UpdateCheCRStatus(ctx, "openShiftOAuthUserCredentialsSecret", ""); err != nil {
			return reconcile.Result{}, false, err
		}

		// set 'openShiftoAuth:nil` to reenable on the following reconcile loop (if possible)
		ctx.CheCluster.Spec.Auth.InitialOpenShiftOAuthUser = nil
		updateFields := map[string]string{
			"openShiftoAuth":            "nil",
			"initialOpenShiftOAuthUser": "nil",
		}

		if err := deploy.UpdateCheCRSpecByFields(ctx, updateFields); err != nil {
			return reconcile.Result{}, false, err
		}

		return reconcile.Result{}, true, nil
	}

	return reconcile.Result{}, true, nil
}

func (oou *OpenShiftOAuthUser) Finalize(ctx *deploy.DeployContext) error {
	if util.IsOpenShift4 {
		return oou.Delete(ctx)
	}

	return nil
}

// Creates new htpasswd provider with initial user with Che flavor name
// if Openshift cluster hasn't got identity providers then does nothing.
// It usefull for good first user experience.
// User can't use kube:admin or system:admin user in the Openshift oAuth when DevWorkspace engine disabled.
// That's why we provide initial user for good first meeting with Eclipse Che.
func (oou *OpenShiftOAuthUser) Create(ctx *deploy.DeployContext) (bool, error) {
	userName := deploy.DefaultCheFlavor(ctx.CheCluster)
	if htpasswdFileContent == "" {
		var err error
		if htpasswdFileContent, err = oou.generateHtPasswdUserInfo(userName, password); err != nil {
			return false, err
		}
	}

	var storedPassword string

	// read existed password from the secret (operator has been restarted case)
	secret, err := oou.getOpenShiftOAuthUserCredentialsSecret(ctx)
	if err != nil {
		return false, err
	} else if secret != nil {
		storedPassword = string(secret.Data["password"])
	}

	if storedPassword != "" && password != storedPassword {
		password = storedPassword
		if htpasswdFileContent, err = oou.generateHtPasswdUserInfo(userName, password); err != nil {
			return false, err
		}
	}

	initialUserSecretData := map[string][]byte{"user": []byte(userName), "password": []byte(password)}
	done, err := deploy.SyncSecretToCluster(ctx, OpenShiftOAuthUserCredentialsSecret, OcConfigNamespace, initialUserSecretData)
	if !done {
		return false, err
	}

	htpasswdFileSecretData := map[string][]byte{"htpasswd": []byte(htpasswdFileContent)}
	done, err = deploy.SyncSecretToCluster(ctx, HtpasswdSecretName, OcConfigNamespace, htpasswdFileSecretData)
	if !done {
		return false, err
	}

	oAuth, err := GetOpenshiftOAuth(ctx)
	if err != nil {
		return false, err
	}

	if err := appendIdentityProvider(oAuth, ctx.ClusterAPI.NonCachedClient); err != nil {
		return false, err
	}

	if err := deploy.AppendFinalizer(ctx, OpenshiftOauthUserFinalizerName); err != nil {
		return false, err
	}

	return true, nil
}

// Removes initial user, htpasswd provider, htpasswd secret and Che secret with username and password.
func (oou *OpenShiftOAuthUser) Delete(ctx *deploy.DeployContext) error {
	oAuth, err := GetOpenshiftOAuth(ctx)
	if err != nil {
		return err
	}

	userName := deploy.DefaultCheFlavor(ctx.CheCluster)
	if _, err := deploy.Delete(ctx, types.NamespacedName{Name: userName}, &userv1.User{}); err != nil {
		return err
	}

	identityName := HtpasswdIdentityProviderName + ":" + userName
	if _, err := deploy.Delete(ctx, types.NamespacedName{Name: identityName}, &userv1.Identity{}); err != nil {
		return err
	}

	if err := deleteIdentityProvider(oAuth, ctx.ClusterAPI.NonCachedClient); err != nil {
		return err
	}

	_, err = deploy.Delete(ctx, types.NamespacedName{Name: HtpasswdSecretName, Namespace: OcConfigNamespace}, &corev1.Secret{})
	if err != nil {
		return err
	}

	// legacy secret in the current namespace
	_, err = deploy.DeleteNamespacedObject(ctx, OpenShiftOAuthUserCredentialsSecret, &corev1.Secret{})
	if err != nil {
		return err
	}

	_, err = deploy.Delete(ctx, types.NamespacedName{Name: OpenShiftOAuthUserCredentialsSecret, Namespace: OcConfigNamespace}, &corev1.Secret{})
	if err != nil {
		return err
	}

	if err := deploy.DeleteFinalizer(ctx, OpenshiftOauthUserFinalizerName); err != nil {
		return err
	}

	return nil
}

func (oou *OpenShiftOAuthUser) generateHtPasswdUserInfo(userName string, password string) (string, error) {
	if util.IsTestMode() {
		return "", nil
	}
	logrus.Info("Generate initial user httpasswd info")

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
	logrus.Info("Add initial user httpasswd provider to the oAuth")

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
