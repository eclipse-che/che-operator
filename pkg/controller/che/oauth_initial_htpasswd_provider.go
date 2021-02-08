//
// Copyright (c) 2012-2020 Red Hat, Inc.
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

	"github.com/eclipse/che-operator/pkg/util"
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
)

type OpenShiftOAuthUserHandler interface {
	CreateOAuthInitialUser(userNamePrefix string, crNamespace string, openshiftOAuth *oauthv1.OAuth) error
	DeleteOAuthInitialUser(userNamePrefix string, crNamespace string) error
}

type OpenShiftOAuthUserOperatorHandler struct {
	OpenShiftOAuthUserHandler
	runtimeClient client.Client
	runnable      util.Runnable
}

func NewOpenShiftOAuthUserHandler(runtimeClient client.Client) OpenShiftOAuthUserHandler {
	return &OpenShiftOAuthUserOperatorHandler{
		runtimeClient: runtimeClient,
		// real process implementation. In the test we are using mock
		runnable: util.NewRunnable(),
	}
}

// CreateOauthInitialUser - creates new htpasswd provider with inital user with name 'che-user'
// if Openshift cluster has got no identity providers, otherwise do nothing.
// It usefull for good first user expirience.
// User can't use kube:admin or system:admin user in the Openshift oAuth. That's why we provide
// initial user for good first meeting with Eclipse Che.
func (handler *OpenShiftOAuthUserOperatorHandler) CreateOauthInitialUser(userNamePrefix string, crNamespace string, openshiftOAuth *oauthv1.OAuth) error {
	password := util.GeneratePasswd(6)

	userName := getUserName(userNamePrefix)
	htpasswdFileContent, err := handler.generateHtPasswdUserInfo(userName, password)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{}
	nsName := types.NamespacedName{Name: htpasswdSecretName, Namespace: ocConfigNamespace}
	if err := handler.runtimeClient.Get(context.TODO(), nsName, secret); err != nil {
		if errors.IsNotFound(err) {
			htpasswdFileSecretData := map[string][]byte{"htpasswd": []byte(htpasswdFileContent)}
			if err = createSecret(htpasswdFileSecretData, htpasswdSecretName, ocConfigNamespace, handler.runtimeClient); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if err := appendIdentityProvider(openshiftOAuth, handler.runtimeClient); err != nil {
		return err
	}

	initialUserSecretData := map[string][]byte{"user": []byte(userName), "password": []byte(password)}
	if err := createSecret(initialUserSecretData, openShiftOAuthUserCredentialsSecret, crNamespace, handler.runtimeClient); err != nil {
		return err
	}

	return nil
}

// DeleteOauthInitialUser removes initial user, htpasswd provider, htpasswd secret and Che secret with username and password.
func (iuh *OpenShiftOAuthUserOperatorHandler) DeleteOauthInitialUser(userNamePrefix string, crNamespace string) error {
	oAuth, err := GetOpenshiftOAuth(iuh.runtimeClient)
	if err != nil {
		return err
	}
	var identityProviderExists bool
	for _, ip := range oAuth.Spec.IdentityProviders {
		if ip.Name == htpasswdIdentityProviderName {
			identityProviderExists = true
			break
		}
	}

	if identityProviderExists {
		userName := getUserName(userNamePrefix)
		if err := deleteSecret(htpasswdSecretName, ocConfigNamespace, iuh.runtimeClient); err != nil {
			return err
		}

		if err := deleteSecret(openShiftOAuthUserCredentialsSecret, crNamespace, iuh.runtimeClient); err != nil {
			return err
		}

		if err := deleteInitialUser(iuh.runtimeClient, userName); err != nil {
			return err
		}

		if err := deleteUserIdentity(iuh.runtimeClient, userName); err != nil {
			return err
		}

		if err := deleteIdentityProvider(oAuth, iuh.runtimeClient); err != nil {
			return err
		}
	}

	return nil
}

func (iuh *OpenShiftOAuthUserOperatorHandler) generateHtPasswdUserInfo(username string, password string) (string, error) {
	logrus.Info("Generate initial user httpasswd info")

	err := iuh.runnable.Run("htpasswd", "-nbB", username, password)
	if err != nil {
		return "", err
	}

	if len(iuh.runnable.GetStdErr()) > 0 {
		return "", errorMsg.New("Failed to generate htpasswd info for initial identity provider: " + iuh.runnable.GetStdErr())
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

func createSecret(content map[string][]byte, secretName string, namespace string, runtimeClient client.Client) error {
	logrus.Info("Create initial user secret for che-operator")
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: content,
	}
	if err := runtimeClient.Create(context.TODO(), secret); err != nil {
		return err
	}
	return nil
}

func appendIdentityProvider(oAuth *oauthv1.OAuth, runtimeClient client.Client) error {
	logrus.Info("Add initial user httpasswd provider to the oAuth")

	oauthPatch := client.MergeFrom(oAuth.DeepCopy())
	htpasswdProvider := newHtpasswdProvider()
	oAuth.Spec.IdentityProviders = append(oAuth.Spec.IdentityProviders, *htpasswdProvider)

	if err := runtimeClient.Patch(context.TODO(), oAuth, oauthPatch); err != nil {
		return err
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

func deleteSecret(secretName string, namespace string, runtimeClient client.Client) error {
	logrus.Infof("Delete secret: %s in the namespace: %s", secretName, namespace)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}

	if err := runtimeClient.Delete(context.TODO(), secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func deleteInitialUser(runtimeClient client.Client, userName string) error {
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

func getUserName(userNamePrefix string) string {
	return userNamePrefix + "-user"
}
