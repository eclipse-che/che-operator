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
	"os"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	util_mocks "github.com/eclipse-che/che-operator/mocks/pkg/util"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/golang/mock/gomock"
	oauth_config "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testNamespace = "test-namespace"
	testUserName  = "test"
)

var (
	testCR = &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: testNamespace,
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheFlavor: testUserName,
			},
		},
	}
)

func TestCreateInitialUser(t *testing.T) {
	type testCase struct {
		name        string
		oAuth       *oauth_config.OAuth
		initObjects []runtime.Object
	}

	oAuth := &oauth_config.OAuth{
		ObjectMeta: v1.ObjectMeta{
			Name: "cluster",
		},
		Spec: oauth_config.OAuthSpec{IdentityProviders: []oauth_config.IdentityProvider{}},
	}

	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	scheme := scheme.Scheme
	orgv1.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
	scheme.AddKnownTypes(oauth_config.SchemeGroupVersion, &oauth_config.OAuth{})

	runtimeClient := fake.NewFakeClientWithScheme(scheme, oAuth, testCR)

	ctrl := gomock.NewController(t)
	m := util_mocks.NewMockRunnable(ctrl)
	m.EXPECT().Run("htpasswd", "-nbB", gomock.Any(), gomock.Any()).Return(nil)
	m.EXPECT().GetStdOut().Return("test-string")
	m.EXPECT().GetStdErr().Return("")
	defer ctrl.Finish()

	initialUserHandler := &OpenShiftOAuthUserOperatorHandler{
		runtimeClient: runtimeClient,
		runnable:      m,
	}
	dc := &deploy.DeployContext{
		CheCluster: testCR,
		ClusterAPI: deploy.ClusterAPI{Client: runtimeClient, NonCachedClient: runtimeClient, DiscoveryClient: nil, Scheme: scheme},
	}
	provisined, err := initialUserHandler.SyncOAuthInitialUser(oAuth, dc)
	if err != nil {
		t.Errorf("Failed to create user: %s", err.Error())
	}
	if !provisined {
		t.Error("Unexpected error")
	}

	// Check created objects
	expectedCheSecret := &corev1.Secret{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: openShiftOAuthUserCredentialsSecret, Namespace: ocConfigNamespace}, expectedCheSecret); err != nil {
		t.Errorf("Initial user secret should exists")
	}

	expectedHtpasswsSecret := &corev1.Secret{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: htpasswdSecretName, Namespace: ocConfigNamespace}, expectedHtpasswsSecret); err != nil {
		t.Errorf("Initial user secret should exists")
	}

	expectedOAuth := &oauth_config.OAuth{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, expectedOAuth); err != nil {
		t.Errorf("Initial oAuth should exists")
	}

	if len(expectedOAuth.Spec.IdentityProviders) < 0 {
		t.Error("List identity providers should not be an empty")
	}

	if !util.ContainsString(testCR.Finalizers, openshiftOauthUserFinalizerName) {
		t.Error("Finaizer hasn't been added")
	}
}

func TestDeleteInitialUser(t *testing.T) {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

	scheme := scheme.Scheme
	orgv1.SchemeBuilder.AddToScheme(scheme)
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.UserList{}, &userv1.User{})
	scheme.AddKnownTypes(oauth_config.SchemeGroupVersion, &oauth_config.OAuth{})
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.Identity{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Secret{})
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, &userv1.User{})

	oAuth := &oauth_config.OAuth{
		ObjectMeta: v1.ObjectMeta{
			Name: "cluster",
		},
		Spec: oauth_config.OAuthSpec{IdentityProviders: []oauth_config.IdentityProvider{*newHtpasswdProvider()}},
	}
	cheSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftOAuthUserCredentialsSecret,
			Namespace: ocConfigNamespace,
		},
	}
	htpasswdSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      htpasswdSecretName,
			Namespace: ocConfigNamespace,
		},
	}
	userIdentity := &userv1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: htpasswdIdentityProviderName + ":" + testUserName,
		},
	}
	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: testUserName,
		},
	}

	runtimeClient := fake.NewFakeClientWithScheme(scheme, oAuth, cheSecret, htpasswdSecret, userIdentity, user, testCR)

	initialUserHandler := &OpenShiftOAuthUserOperatorHandler{
		runtimeClient: runtimeClient,
	}

	dc := &deploy.DeployContext{
		CheCluster: testCR,
		ClusterAPI: deploy.ClusterAPI{Client: runtimeClient, NonCachedClient: runtimeClient, DiscoveryClient: nil, Scheme: scheme},
	}
	if err := initialUserHandler.DeleteOAuthInitialUser(dc); err != nil {
		t.Errorf("Unable to delete initial user: %s", err.Error())
	}

	expectedCheSecret := &corev1.Secret{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: openShiftOAuthUserCredentialsSecret, Namespace: ocConfigNamespace}, expectedCheSecret); !errors.IsNotFound(err) {
		t.Errorf("Initial user secret should be deleted")
	}

	expectedHtpasswsSecret := &corev1.Secret{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: htpasswdSecretName, Namespace: ocConfigNamespace}, expectedHtpasswsSecret); !errors.IsNotFound(err) {
		t.Errorf("Initial user secret should be deleted")
	}

	expectedUserIdentity := &userv1.Identity{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: htpasswdIdentityProviderName + ":" + testUserName}, expectedUserIdentity); !errors.IsNotFound(err) {
		t.Errorf("Initial user identity should be deleted")
	}

	expectedUser := &userv1.User{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: testUserName}, expectedUser); !errors.IsNotFound(err) {
		t.Errorf("Initial user should be deleted")
	}

	expectedOAuth := &oauth_config.OAuth{}
	if err := runtimeClient.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, expectedOAuth); err != nil {
		t.Errorf("OAuth should exists")
	}

	if len(expectedOAuth.Spec.IdentityProviders) != 0 {
		t.Error("List identity providers should be an empty")
	}

	if util.ContainsString(testCR.Finalizers, openshiftOauthUserFinalizerName) {
		t.Error("Finalizer hasn't been removed")
	}
}
