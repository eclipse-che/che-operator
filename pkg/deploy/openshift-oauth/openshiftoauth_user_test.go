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
	"os"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	oauth_config "github.com/openshift/api/config/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testNamespace = "test-namespace"
	testUserName  = "test"
)

func TestCreateInitialUser(t *testing.T) {
	checluster := &orgv1.CheCluster{
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
	oAuth := &oauth_config.OAuth{
		ObjectMeta: v1.ObjectMeta{
			Name: "cluster",
		},
		Spec: oauth_config.OAuthSpec{IdentityProviders: []oauth_config.IdentityProvider{}},
	}
	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
	ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{oAuth})

	openShiftOAuthUser := NewOpenShiftOAuthUser()
	done, err := openShiftOAuthUser.Create(ctx)
	assert.Nil(t, err)
	assert.True(t, done)

	// Check created objects
	expectedCheSecret := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: OpenShiftOAuthUserCredentialsSecret, Namespace: OcConfigNamespace}, expectedCheSecret)
	assert.Nil(t, err)

	expectedHtpasswsSecret := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: HtpasswdSecretName, Namespace: OcConfigNamespace}, expectedHtpasswsSecret)
	assert.Nil(t, err)

	expectedOAuth := &oauth_config.OAuth{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, expectedOAuth)
	assert.Nil(t, err)
	assert.Equal(t, len(expectedOAuth.Spec.IdentityProviders), 1)
	assert.True(t, util.ContainsString(checluster.Finalizers, OpenshiftOauthUserFinalizerName))

	assert.Equal(t, checluster.Status.OpenShiftOAuthUserCredentialsSecret, OpenShiftOAuthUserCredentialsSecret)
}

func TestDeleteInitialUser(t *testing.T) {
	checluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: testNamespace,
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheFlavor: testUserName,
			},
		},
		Status: orgv1.CheClusterStatus{
			OpenShiftOAuthUserCredentialsSecret: "some-secret",
		},
	}
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
			Name:      OpenShiftOAuthUserCredentialsSecret,
			Namespace: OcConfigNamespace,
		},
	}
	htpasswdSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      HtpasswdSecretName,
			Namespace: OcConfigNamespace,
		},
	}
	userIdentity := &userv1.Identity{
		ObjectMeta: metav1.ObjectMeta{
			Name: HtpasswdIdentityProviderName + ":" + testUserName,
		},
	}
	user := &userv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: testUserName,
		},
	}

	logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
	ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{oAuth, cheSecret, htpasswdSecret, userIdentity, user})

	openShiftOAuthUser := &OpenShiftOAuthUser{}
	err := openShiftOAuthUser.Delete(ctx)
	assert.Nil(t, err)

	expectedCheSecret := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: OpenShiftOAuthUserCredentialsSecret, Namespace: OcConfigNamespace}, expectedCheSecret)
	assert.True(t, errors.IsNotFound(err))

	expectedHtpasswsSecret := &corev1.Secret{}
	if err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: HtpasswdSecretName, Namespace: OcConfigNamespace}, expectedHtpasswsSecret); !errors.IsNotFound(err) {
		t.Errorf("Initial user secret should be deleted")
	}
	assert.True(t, errors.IsNotFound(err))

	expectedUserIdentity := &userv1.Identity{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: HtpasswdIdentityProviderName + ":" + testUserName}, expectedUserIdentity)
	assert.True(t, errors.IsNotFound(err))

	expectedUser := &userv1.User{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: testUserName}, expectedUser)
	assert.True(t, errors.IsNotFound(err))

	expectedOAuth := &oauth_config.OAuth{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, expectedOAuth)
	assert.Nil(t, err)
	assert.Equal(t, len(expectedOAuth.Spec.IdentityProviders), 0)
	assert.False(t, util.ContainsString(checluster.Finalizers, OpenshiftOauthUserFinalizerName))
	assert.Empty(t, checluster.Status.OpenShiftOAuthUserCredentialsSecret)
}
