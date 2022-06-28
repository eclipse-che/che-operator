//
// Copyright (c) 2019-2022 Red Hat, Inc.
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
	"os"
	"testing"

	"k8s.io/utils/pointer"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	oauthv1 "github.com/openshift/api/oauth/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestFinalize(t *testing.T) {
	oauthClient1 := GetOAuthClientSpec("test1", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)
	oauthClient2 := GetOAuthClientSpec("test2", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)
	oauthClient3 := GetOAuthClientSpec("test3", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)
	oauthClient3.ObjectMeta.Labels = map[string]string{}

	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "eclipse-che",
			Namespace:  "eclipse-che",
			Finalizers: []string{OAuthFinalizerName},
		},
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{oauthClient1, oauthClient2, oauthClient3})

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test1"}, &oauthv1.OAuthClient{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test2"}, &oauthv1.OAuthClient{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test3"}, &oauthv1.OAuthClient{}))

	identityProviderReconciler := NewIdentityProviderReconciler()
	done := identityProviderReconciler.Finalize(ctx)
	assert.True(t, done)
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test1"}, &oauthv1.OAuthClient{}))
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test2"}, &oauthv1.OAuthClient{}))
	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test3"}, &oauthv1.OAuthClient{}))
	assert.Equal(t, 0, len(checluster.Finalizers))
}

func TestSyncOAuthClientShouldSyncTokenTimeout(t *testing.T) {
	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthAccessTokenInactivityTimeoutSeconds: pointer.Int32Ptr(10),
					OAuthAccessTokenMaxAgeSeconds:            pointer.Int32Ptr(20),
				},
			},
		},
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{})
	done, err := syncOAuthClient(ctx)
	assert.True(t, done)
	assert.Nil(t, err)

	oauthClients, err := FindAllEclipseCheOAuthClients(ctx)
	assert.Nil(t, err)
	assert.Equal(t, int32(10), *oauthClients[0].AccessTokenInactivityTimeoutSeconds)
	assert.Equal(t, int32(20), *oauthClients[0].AccessTokenMaxAgeSeconds)
}

func TestSyncOAuthClient(t *testing.T) {
	type testCase struct {
		name           string
		cheCluster     *chev2.CheCluster
		expectedName   string
		expectedSecret string
	}

	testCases := []testCase{
		{
			name: "Sync OAuthClient",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthSecret:     "secret",
							OAuthClientName: "test",
						},
					},
				},
			},
			expectedName:   "test",
			expectedSecret: "secret",
		},
		{
			name: "Sync OAuthClient, generate name and secret",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name: "Sync OAuthClient, generate secret",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthClientName: "test",
						},
					},
				},
			},
			expectedName: "test",
		},
		{
			name: "Sync OAuthClient, generate name",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthSecret: "secret",
						},
					},
				},
			},
			expectedSecret: "secret",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			_, err := syncOAuthClient(ctx)
			assert.Nil(t, err)

			oauthClients, err := FindAllEclipseCheOAuthClients(ctx)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(oauthClients))
			if testCase.expectedName != "" {
				assert.Equal(t, testCase.expectedName, oauthClients[0].Name)
			}
			if testCase.expectedSecret != "" {
				assert.Equal(t, testCase.expectedSecret, oauthClients[0].Secret)
			}
		})
	}
}

func TestSyncExistedOAuthClient(t *testing.T) {
	oauthClient := GetOAuthClientSpec("test", "secret", []string{}, nil, nil)

	type testCase struct {
		name           string
		cheCluster     *chev2.CheCluster
		expectedName   string
		expectedSecret string
	}

	testCases := []testCase{
		{
			name: "Sync existed OAuthClient",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedSecret: "secret",
			expectedName:   "test",
		},
		{
			name: "Sync existed OAuthClient, OAuthSecret and OAuthClientName are defined",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthSecret:     "secret",
							OAuthClientName: "test",
						},
					},
				},
			},
			expectedSecret: "secret",
			expectedName:   "test",
		},
		{
			name: "Sync existed OAuthClient, OAuthClientName is defined",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthClientName: "test",
						},
					},
				},
			},
			expectedSecret: "secret",
			expectedName:   "test",
		},
		{
			name: "Sync existed OAuthClient, OAuthSecret is defined",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthSecret: "secret",
						},
					},
				},
			},
			expectedSecret: "secret",
			expectedName:   "test",
		},
		{
			name: "Sync existed OAuthClient, update secret, usecase #1",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthSecret: "new-secret",
						},
					},
				},
			},
			expectedSecret: "new-secret",
			expectedName:   "test",
		},
		{
			name: "Sync existed OAuthClient, update secret, usecase #2",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Networking: chev2.CheClusterSpecNetworking{
						Auth: chev2.Auth{
							OAuthClientName: "test",
							OAuthSecret:     "new-secret",
						},
					},
				},
			},
			expectedSecret: "new-secret",
			expectedName:   "test",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{oauthClient})
			_, err := syncOAuthClient(ctx)
			assert.Nil(t, err)

			oauthClients, err := FindAllEclipseCheOAuthClients(ctx)
			assert.Nil(t, err)
			assert.Equal(t, 1, len(oauthClients))
			if testCase.expectedName != "" {
				assert.Equal(t, testCase.expectedName, oauthClients[0].Name)
			}
			if testCase.expectedSecret != "" {
				assert.Equal(t, testCase.expectedSecret, oauthClients[0].Secret)
			}
		})
	}
}
