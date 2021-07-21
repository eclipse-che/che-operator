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

package che

import (
	"context"

	errorMsg "errors"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	htpasswdIdentityProviderName        = "htpasswd-eclipse-che"
	htpasswdSecretName                  = "htpasswd-eclipse-che"
	ocConfigNamespace                   = "openshift-config"
	openShiftOAuthUserCredentialsSecret = "openshift-oauth-user-credentials"
	openshiftOauthUserFinalizerName     = "openshift-oauth-user.finalizers.che.eclipse.org"
)

var (
	password            = util.GeneratePasswd(6)
	htpasswdFileContent string
)

// OpenShiftOAuthUserHandler - handler to create or delete new Openshift oAuth user.
type OpenShiftOAuthUserHandler interface {
	SyncOAuthInitialUser(openshiftOAuth *oauthv1.OAuth, deployContext *deploy.DeployContext) (bool, error)
	DeleteOAuthInitialUser(deployContext *deploy.DeployContext) error
}

// OpenShiftOAuthUserOperatorHandler - OpenShiftOAuthUserHandler implementation.
type OpenShiftOAuthUserOperatorHandler struct {
	OpenShiftOAuthUserHandler
	runtimeClient client.Client
	runnable      util.Runnable
}

// NewOpenShiftOAuthUserHandler - create new OpenShiftOAuthUserHandler instance
func NewOpenShiftOAuthUserHandler(runtimeClient client.Client) OpenShiftOAuthUserHandler {
	return &OpenShiftOAuthUserOperatorHandler{
		runtimeClient: runtimeClient,
		// real process implementation. In the test we are using mock.
		runnable: util.NewRunnable(),
	}
}

// SyncOAuthInitialUser - creates new htpasswd provider with initial user with Che flavor name
// if Openshift cluster hasn't got identity providers, otherwise do nothing.
// It usefull for good first user experience.
// User can't use kube:admin or system:admin user in the Openshift oAuth. That's why we provide
// initial user for good first meeting with Eclipse Che.
func (iuh *OpenShiftOAuthUserOperatorHandler) SyncOAuthInitialUser(openshiftOAuth *oauthv1.OAuth, deployContext *deploy.DeployContext) (bool, error) {
	cr := deployContext.CheCluster
	userName := deploy.DefaultCheFlavor(cr)
	if htpasswdFileContent == "" {
		var err error
		if htpasswdFileContent, err = iuh.generateHtPasswdUserInfo(userName, password); err != nil {
			return false, err
		}
	}

	var storedPassword string

	// read existed password from the secret (operator has been restarted case)
	secret := &corev1.Secret{}
	exists, err := getOpenShiftOAuthUserCredentialsSecret(deployContext, secret)
	if err != nil {
		return false, err
	} else if exists {
		storedPassword = string(secret.Data["password"])
	}

	if storedPassword != "" && password != storedPassword {
		password = storedPassword
		if htpasswdFileContent, err = iuh.generateHtPasswdUserInfo(userName, password); err != nil {
			return false, err
		}
	}

	initialUserSecretData := map[string][]byte{"user": []byte(userName), "password": []byte(password)}
	done, err := deploy.SyncSecretToCluster(deployContext, openShiftOAuthUserCredentialsSecret, ocConfigNamespace, initialUserSecretData)
	if !done {
		return false, err
	}

	htpasswdFileSecretData := map[string][]byte{"htpasswd": []byte(htpasswdFileContent)}
	done, err = deploy.SyncSecretToCluster(deployContext, htpasswdSecretName, ocConfigNamespace, htpasswdFileSecretData)
	if !done {
		return false, err
	}

	if err := appendIdentityProvider(openshiftOAuth, iuh.runtimeClient); err != nil {
		return false, err
	}

	if err := deploy.AppendFinalizer(deployContext, openshiftOauthUserFinalizerName); err != nil {
		return false, err
	}

	return true, nil
}

// DeleteOAuthInitialUser - removes initial user, htpasswd provider, htpasswd secret and Che secret with username and password.
func (iuh *OpenShiftOAuthUserOperatorHandler) DeleteOAuthInitialUser(deployContext *deploy.DeployContext) error {
	oAuth, err := GetOpenshiftOAuth(iuh.runtimeClient)
	if err != nil {
		return err
	}

	cr := deployContext.CheCluster
	userName := deploy.DefaultCheFlavor(cr)

	if err := deleteUser(iuh.runtimeClient, userName); err != nil {
		return err
	}

	if err := deleteUserIdentity(iuh.runtimeClient, userName); err != nil {
		return err
	}

	if err := deleteIdentityProvider(oAuth, iuh.runtimeClient); err != nil {
		return err
	}

	_, err = deploy.Delete(deployContext, types.NamespacedName{Name: htpasswdSecretName, Namespace: ocConfigNamespace}, &corev1.Secret{})
	if err != nil {
		return err
	}

	// legacy secret in the current namespace
	_, err = deploy.DeleteNamespacedObject(deployContext, openShiftOAuthUserCredentialsSecret, &corev1.Secret{})
	if err != nil {
		return err
	}

	_, err = deploy.Delete(deployContext, types.NamespacedName{Name: openShiftOAuthUserCredentialsSecret, Namespace: ocConfigNamespace}, &corev1.Secret{})
	if err != nil {
		return err
	}

	if err := deploy.DeleteFinalizer(deployContext, openshiftOauthUserFinalizerName); err != nil {
		return err
	}

	return nil
}

func (iuh *OpenShiftOAuthUserOperatorHandler) generateHtPasswdUserInfo(userName string, password string) (string, error) {
	logrus.Info("Generate initial user httpasswd info")

	err := iuh.runnable.Run("htpasswd", "-nbB", userName, password)
	if err != nil {
		return "", err
	}

	if len(iuh.runnable.GetStdErr()) > 0 {
		return "", errorMsg.New("Failed to generate data for HTPasswd identity provider: " + iuh.runnable.GetStdErr())
	}
	return iuh.runnable.GetStdOut(), nil
}

// GetOpenshiftOAuth returns Openshift oAuth object.
func GetOpenshiftOAuth(runtimeClient client.Client) (*oauthv1.OAuth, error) {
	oAuth := &oauthv1.OAuth{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, oAuth); err != nil {
		return nil, err
	}
	return oAuth, nil
}

func identityProviderExists(providerName string, oAuth *oauthv1.OAuth) bool {
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

// read from the secret from `openshift-config` namespace
// and then from the legacy secret from the current namespace
func getOpenShiftOAuthUserCredentialsSecret(deployContext *deploy.DeployContext, secret *corev1.Secret) (bool, error) {
	exists, err := deploy.Get(deployContext, types.NamespacedName{Name: openShiftOAuthUserCredentialsSecret, Namespace: ocConfigNamespace}, secret)
	if err != nil {
		return false, err
	} else if exists {
		return true, nil
	}

	return deploy.GetNamespacedObject(deployContext, openShiftOAuthUserCredentialsSecret, secret)
}

func appendIdentityProvider(oAuth *oauthv1.OAuth, runtimeClient client.Client) error {
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

func newHtpasswdProvider() *oauthv1.IdentityProvider {
	return &oauthv1.IdentityProvider{
		Name:          htpasswdIdentityProviderName,
		MappingMethod: configv1.MappingMethodClaim,
		IdentityProviderConfig: oauthv1.IdentityProviderConfig{
			Type: "HTPasswd",
			HTPasswd: &oauthv1.HTPasswdIdentityProvider{
				FileData: oauthv1.SecretNameReference{Name: htpasswdSecretName},
			},
		},
	}
}

func deleteUser(runtimeClient client.Client, userName string) error {
	logrus.Infof("Delete initial user: %s", userName)

	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: userName,
		},
	}

	if err := runtimeClient.Delete(context.TODO(), user); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func deleteUserIdentity(runtimeClient client.Client, userName string) error {
	identityName := htpasswdIdentityProviderName + ":" + userName
	logrus.Infof("Delete initial user identity: %s", identityName)

	identity := &userv1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: identityName,
		},
	}

	if err := runtimeClient.Delete(context.TODO(), identity); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func deleteIdentityProvider(oAuth *configv1.OAuth, runtimeClient client.Client) error {
	logrus.Info("Delete initial user httpasswd provider from the oAuth")

	oauthPatch := client.MergeFrom(oAuth.DeepCopy())
	ips := oAuth.Spec.IdentityProviders
	for i, ip := range ips {
		if ip.Name == htpasswdIdentityProviderName {
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
