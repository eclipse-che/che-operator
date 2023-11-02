//
// Copyright (c) 2019-2023 Red Hat, Inc.
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

func TestFinalizeDefaultOAuthClientName(t *testing.T) {
	oauthClient := GetOAuthClientSpec("eclipse-che-client", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)
	oauthClient.ObjectMeta.Labels = map[string]string{}

	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "eclipse-che",
			Namespace:  "eclipse-che",
			Finalizers: []string{OAuthFinalizerName},
		},
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{oauthClient})

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "eclipse-che-client"}, &oauthv1.OAuthClient{}))

	identityProviderReconciler := NewIdentityProviderReconciler()
	done := identityProviderReconciler.Finalize(ctx)
	assert.True(t, done)
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "eclipse-che-client"}, &oauthv1.OAuthClient{}))
	assert.Equal(t, 0, len(checluster.Finalizers))
}

func TestFinalizeOAuthClient(t *testing.T) {
	oauthClient := GetOAuthClientSpec("test", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)
	oauthClient.ObjectMeta.Labels = map[string]string{}

	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "eclipse-che",
			Namespace:  "eclipse-che",
			Finalizers: []string{OAuthFinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthClientName: "test",
				},
			},
		},
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{oauthClient})

	assert.True(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test"}, &oauthv1.OAuthClient{}))

	identityProviderReconciler := NewIdentityProviderReconciler()
	done := identityProviderReconciler.Finalize(ctx)
	assert.True(t, done)
	assert.False(t, test.IsObjectExists(ctx.ClusterAPI.Client, types.NamespacedName{Name: "test"}, &oauthv1.OAuthClient{}))
	assert.Equal(t, 0, len(checluster.Finalizers))
}

func TestShouldFindSingleOAuthClient(t *testing.T) {
	oauthClient1 := GetOAuthClientSpec("test1", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)
	oauthClient2 := GetOAuthClientSpec("test2", "secret", []string{"https://che-host/oauth/callback"}, nil, nil)

	checluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Auth: chev2.Auth{
					OAuthClientName: "test1",
				},
			},
		},
	}

	ctx := test.GetDeployContext(checluster, []runtime.Object{oauthClient1, oauthClient2})
	oauthClient, err := GetOAuthClient(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, oauthClient)
	assert.Equal(t, "test1", oauthClient.Name)
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

	oauthClient, err := GetOAuthClient(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, oauthClient)
	assert.Equal(t, int32(10), *oauthClient.AccessTokenInactivityTimeoutSeconds)
	assert.Equal(t, int32(20), *oauthClient.AccessTokenMaxAgeSeconds)
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
			name: "Sync OAuthClient, generate secret, default name",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedName: "eclipse-che-client",
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
			name: "Sync OAuthClient, default name",
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
			expectedName:   "eclipse-che-client",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})
			_, err := syncOAuthClient(ctx)
			assert.Nil(t, err)

			oauthClient, err := GetOAuthClient(ctx)
			assert.Nil(t, err)
			assert.NotNil(t, oauthClient)
			if testCase.expectedName != "" {
				assert.Equal(t, testCase.expectedName, oauthClient.Name)
			}
			if testCase.expectedSecret != "" {
				assert.Equal(t, testCase.expectedSecret, oauthClient.Secret)
			}
		})
	}
}

func TestSyncExistedOAuthClient(t *testing.T) {
	oauthClient1 := GetOAuthClientSpec("test", "secret", []string{}, nil, nil)
	oauthClient2 := GetOAuthClientSpec("eclipse-che-client", "secret", []string{}, nil, nil)

	type testCase struct {
		name           string
		cheCluster     *chev2.CheCluster
		expectedName   string
		expectedSecret string
	}

	testCases := []testCase{
		{
			name: "Sync existed OAuthClient with the default name",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedSecret: "secret",
			expectedName:   "eclipse-che-client",
		},
		{
			name: "Sync existed OAuthClient with a custom name",
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
			name: "Sync existed OAuthClient with a custom name, update secret",
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
		{
			name: "Sync existed OAuthClient with the default name, update secret",
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
			expectedName:   "eclipse-che-client",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{oauthClient1, oauthClient2})
			_, err := syncOAuthClient(ctx)
			assert.Nil(t, err)

			oauthClient, err := GetOAuthClient(ctx)
			assert.Nil(t, err)
			assert.NotNil(t, oauthClient)
			if testCase.expectedName != "" {
				assert.Equal(t, testCase.expectedName, oauthClient.Name)
			}
			if testCase.expectedSecret != "" {
				assert.Equal(t, testCase.expectedSecret, oauthClient.Secret)
			}
		})
	}
}
