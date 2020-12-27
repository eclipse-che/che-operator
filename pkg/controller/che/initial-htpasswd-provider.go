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
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	htpasswdIdentityProviderName = "htpasswd-eclipse"
	htpasswdSecretName           = "htpasswd-eclipse"
	ocConfigNamespace            = "openshift-config"
)

// HandleOauthInitialUser - creates new htpasswd provider with inital user with name 'user'
// if Openshift cluster has got no identity providers, otherwise do nothing.
// It usefull for good first user expirience.
// User can't use kube:admin or system:admin user in the Openshift oAuth. That's why we provide
// initial user for good first meeting with Eclipse Che.
func HandleOauthInitialUser(runtimeClient client.Client) error {
	currentOAuth := &oauthv1.OAuth{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, currentOAuth); err != nil {
		return err
	}

	if len(currentOAuth.Spec.IdentityProviders) < 1 {
		secret := &corev1.Secret{}
		nsName := types.NamespacedName{Name: htpasswdSecretName, Namespace: ocConfigNamespace}
		if err := runtimeClient.Get(context.TODO(), nsName, secret); err != nil {
			if errors.IsNotFound(err) {
				if err = createNewHtpasswdSecret(runtimeClient); err != nil {
					return err
				}
			} else {
				return err
			}
		}

		if err := appendIdentityProvider(currentOAuth, runtimeClient); err != nil {
			return err
		}
	}

	return nil
}

func createNewHtpasswdSecret(runtimeClient client.Client) error {
	htpasswdFileContent, err := generateHtPasswdUserInfo()
	if err != nil {
		return err
	}

	logrus.Info("Create initial user httpasswd secret")

	secret := newUserHtpasswdSecret([]byte(htpasswdFileContent))
	if err = runtimeClient.Create(context.TODO(), secret); err != nil {
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

func newUserHtpasswdSecret(htpasswdData []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      htpasswdSecretName,
			Namespace: ocConfigNamespace,
		},
		Data: map[string][]byte{"htpasswd": htpasswdData},
	}
}

func newHtpasswdProvider() *oauthv1.IdentityProvider {
	return &oauthv1.IdentityProvider{
		Name:          htpasswdIdentityProviderName,
		MappingMethod: configv1.MappingMethodClaim,
		IdentityProviderConfig: oauthv1.IdentityProviderConfig{
			Type: "HTPasswd",
			HTPasswd: &oauthv1.HTPasswdIdentityProvider{
				FileData: oauthv1.SecretNameReference{Name: htpasswdSecretName}, // htpasswdSecretName
			},
		},
	}
}

func generateHtPasswdUserInfo() (string, error) {
	logrus.Info("Generate initial user httpasswd info")

	command := util.NewUserCmd
	err := command.Run()
	if err != nil {
		return "", err
	}

	if len(command.GetStdErr()) > 0 {
		return "", errorMsg.New("Failed to generate htpasswd info for initial identity provider: " + command.GetStdErr())
	}
	return command.GetStdOut(), nil
}
