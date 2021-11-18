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
package identity_provider

import (
	"os"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	"github.com/google/go-cmp/cmp"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"testing"
)

func TestSyncGitHubOAuth(t *testing.T) {
	type testCase struct {
		name        string
		initCR      *orgv1.CheCluster
		expectedCR  *orgv1.CheCluster
		initObjects []runtime.Object
	}

	testCases := []testCase{
		{
			name: "Should provision GitHub OAuth with legacy secret",
			initCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "0",
				},
			},
			expectedCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "1",
				},
				Status: orgv1.CheClusterStatus{
					GitHubOAuthProvisioned: true,
				},
			},
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "github-credentials",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
							deploy.KubernetesComponentLabelKey: "keycloak-secret",
						},
						Annotations: map[string]string{
							deploy.CheEclipseOrgGithubOAuthCredentials: "true",
						},
					},
					Data: map[string][]byte{
						"key": []byte("key-data"),
					},
				},
			},
		},
		{
			name: "Should provision GitHub OAuth",
			initCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "0",
				},
			},
			expectedCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "1",
				},
				Status: orgv1.CheClusterStatus{
					GitHubOAuthProvisioned: true,
				},
			},
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "github-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server": "github",
						},
					},
				},
			},
		},
		{
			name: "Should not provision GitHub OAuth",
			initCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "0",
				},
			},
			expectedCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "0",
				},
			},
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "github-credentials",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							deploy.KubernetesPartOfLabelKey:    deploy.CheEclipseOrg,
							deploy.KubernetesComponentLabelKey: "keycloak-secret",
						},
					},
					Data: map[string][]byte{
						"key": []byte("key-data"),
					},
				},
			},
		},
		{
			name: "Should delete GitHub OAuth",
			initCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "0",
				},
				Status: orgv1.CheClusterStatus{
					GitHubOAuthProvisioned: true,
				},
			},
			expectedCR: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che-cluster",
					Namespace:       "eclipse-che",
					ResourceVersion: "1",
				},
				Status: orgv1.CheClusterStatus{
					GitHubOAuthProvisioned: false,
				},
			},
			initObjects: []runtime.Object{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initCR)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.initCR,
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
			}

			_, err := SyncGitHubOAuth(deployContext)
			if err != nil {
				t.Fatalf("Error mounting secret: %v", err)
			}

			if !reflect.DeepEqual(testCase.expectedCR, testCase.initCR) {
				t.Errorf("Expected CR and CR returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedCR, testCase.initCR))
			}
		})
	}
}
