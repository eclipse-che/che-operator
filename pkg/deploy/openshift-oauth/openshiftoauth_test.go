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
	"os"
	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	nonEmptyUserList = &userv1.UserList{
		Items: []userv1.User{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user2",
				},
			},
		},
	}
	oAuthWithNoIdentityProviders = &configv1.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}
	oAuthWithIdentityProvider = &configv1.OAuth{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
		Spec: configv1.OAuthSpec{
			IdentityProviders: []configv1.IdentityProvider{
				{
					Name: "htpasswd",
				},
			},
		},
	}
	proxy = &configv1.Proxy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
		},
	}
)

func TestCaseAutoDetectOAuth(t *testing.T) {
	type testCase struct {
		name                             string
		initObjects                      []runtime.Object
		isOpenshift4                     bool
		initialOAuthValue                *bool
		oAuthExpected                    *bool
		initialOpenShiftOAuthUserEnabled *bool
	}

	testCases := []testCase{
		{
			name: "che-operator should auto enable oAuth when Che CR with oAuth nil value on the Openshift 3 with users > 0",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauthv1.OAuthClient{},
			},
			isOpenshift4:      false,
			initialOAuthValue: nil,
			oAuthExpected:     pointer.BoolPtr(true),
		},
		{
			name: "che-operator should auto disable oAuth when Che CR with nil oAuth on the Openshift 3 with no users",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauthv1.OAuthClient{},
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(false),
			oAuthExpected:     pointer.BoolPtr(false),
		},
		{
			name: "che-operator should respect oAuth = true even if there no users on the Openshift 3",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauthv1.OAuthClient{},
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(true),
			oAuthExpected:     pointer.BoolPtr(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there are some users on the Openshift 3",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauthv1.OAuthClient{},
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(true),
			oAuthExpected:     pointer.BoolPtr(true),
		},
		{
			name: "che-operator should respect oAuth = false even if there are some users on the Openshift 3",
			initObjects: []runtime.Object{
				nonEmptyUserList,
				&oauthv1.OAuthClient{},
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(false),
			oAuthExpected:     pointer.BoolPtr(false),
		},
		{
			name: "che-operator should respect oAuth = false even if no users on the Openshift 3",
			initObjects: []runtime.Object{
				&userv1.UserList{},
				&oauthv1.OAuthClient{},
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(false),
			oAuthExpected:     pointer.BoolPtr(false),
		},
		{
			name: "che-operator should auto enable oAuth when Che CR with nil value on the Openshift 4 with identity providers",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
				proxy,
			},
			isOpenshift4:      true,
			initialOAuthValue: nil,
			oAuthExpected:     pointer.BoolPtr(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
				proxy,
			},
			isOpenshift4:                     true,
			initialOAuthValue:                pointer.BoolPtr(true),
			oAuthExpected:                    pointer.BoolPtr(true),
			initialOpenShiftOAuthUserEnabled: pointer.BoolPtr(true),
		},
		{
			name: "che-operator should respect oAuth = true even if there are some users on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
				proxy,
			},
			isOpenshift4:                     false,
			initialOAuthValue:                pointer.BoolPtr(true),
			oAuthExpected:                    pointer.BoolPtr(true),
			initialOpenShiftOAuthUserEnabled: pointer.BoolPtr(true),
		},
		{
			name: "che-operator should respect oAuth = false even if there no indentity providers on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithNoIdentityProviders,
				proxy,
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(false),
			oAuthExpected:     pointer.BoolPtr(false),
		},
		{
			name: "che-operator should respect oAuth = false even if there are some users on the Openshift 4",
			initObjects: []runtime.Object{
				oAuthWithIdentityProvider,
				proxy,
			},
			isOpenshift4:      false,
			initialOAuthValue: pointer.BoolPtr(false),
			oAuthExpected:     pointer.BoolPtr(false),
		},
		{
			name:                             "che-operator should auto disable oAuth on error retieve identity providers",
			initObjects:                      []runtime.Object{},
			isOpenshift4:                     false,
			initialOAuthValue:                nil,
			initialOpenShiftOAuthUserEnabled: pointer.BoolPtr(true),
			oAuthExpected:                    pointer.BoolPtr(false),
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			checluster := &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      os.Getenv("CHE_FLAVOR"),
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Auth: orgv1.CheClusterSpecAuth{
						OpenShiftoAuth:            testCase.initialOAuthValue,
						InitialOpenShiftOAuthUser: testCase.initialOpenShiftOAuthUserEnabled,
					},
				},
			}

			util.IsOpenShift = true
			util.IsOpenShift4 = testCase.isOpenshift4
			deployContext := deploy.GetTestDeployContext(checluster, testCase.initObjects)

			openShiftOAuth := NewOpenShiftOAuth(NewOpenShiftOAuthUser())
			_, done, err := openShiftOAuth.Reconcile(deployContext)
			assert.Nil(t, err)
			assert.True(t, done)

			assert.NotNil(t, deployContext.CheCluster.Spec.Auth.OpenShiftoAuth)
			assert.Equal(t, *testCase.oAuthExpected, *deployContext.CheCluster.Spec.Auth.OpenShiftoAuth)
		})
	}
}
