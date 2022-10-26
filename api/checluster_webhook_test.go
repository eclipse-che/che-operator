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

package org

import (
	"context"
	"testing"

	"k8s.io/utils/pointer"

	v2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetVSXUrl(t *testing.T) {
	type testCase struct {
		name               string
		cheCluster         *v2.CheCluster
		expectedOpenVSXUrl string
	}

	testCases := []testCase{
		{
			name: "Should set openVSXURL",
			cheCluster: &v2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
			},
			expectedOpenVSXUrl: constants.DefaultOpenVSXUrl,
		},
		{
			name: "Should not set openVSXURL in disconnected environment",
			cheCluster: &v2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: v2.CheClusterSpec{
					ContainerRegistry: v2.CheClusterContainerRegistry{
						Hostname: "hostname",
					},
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should not update openVSXURL for next version",
			cheCluster: &v2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: v2.CheClusterStatus{
					CheVersion: "next",
				},
			},
			expectedOpenVSXUrl: "",
		},
		{
			name: "Should set default openVSXURL if Eclipse Che version is less then 7.53.0",
			cheCluster: &v2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: v2.CheClusterStatus{
					CheVersion: "7.52.2",
				},
			},
			expectedOpenVSXUrl: constants.DefaultOpenVSXUrl,
		},
		{
			name: "Should not update openVSXURL if Eclipse Che version is less then 7.53.0",
			cheCluster: &v2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Spec: v2.CheClusterSpec{
					Components: v2.CheClusterComponents{
						PluginRegistry: v2.PluginRegistry{
							OpenVSXURL: pointer.StringPtr("open-vsx-url"),
						},
					},
				},
				Status: v2.CheClusterStatus{
					CheVersion: "7.52.2",
				},
			},
			expectedOpenVSXUrl: "open-vsx-url",
		},
		{
			name: "Should not update default openVSXURL if Eclipse Che version is greater or equal to 7.53.0",
			cheCluster: &v2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "eclipse-che",
					Namespace: "eclipse-che",
				},
				Status: v2.CheClusterStatus{
					CheVersion: "7.53.0",
				},
			},
			expectedOpenVSXUrl: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testCase.cheCluster.Default()
			if testCase.expectedOpenVSXUrl == "" {
				assert.True(t, testCase.cheCluster.IsOpenVSXURLEmpty())
			} else {
				assert.Equal(t, testCase.expectedOpenVSXUrl, *testCase.cheCluster.Spec.Components.PluginRegistry.OpenVSXURL)
			}
		})
	}
}

func TestValidateScmSecrets(t *testing.T) {
	k8sHelper := k8shelper.New()

	githubSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "github-scm-secret",
		},
		Data: map[string][]byte{
			"id":     []byte("id"),
			"secret": []byte("secret"),
		},
	}
	_, err := k8sHelper.GetClientset().CoreV1().Secrets("eclipse-che").Create(context.TODO(), githubSecret, metav1.CreateOptions{})
	assert.Nil(t, err)

	gitlabSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "gitlab-scm-secret",
		},
		Data: map[string][]byte{
			"id":     []byte("id"),
			"secret": []byte("secret"),
		},
	}
	_, err = k8sHelper.GetClientset().CoreV1().Secrets("eclipse-che").Create(context.TODO(), gitlabSecret, metav1.CreateOptions{})
	assert.Nil(t, err)

	bitbucketSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "bitbucket-scm-secret",
		},
		Data: map[string][]byte{
			"private.key":  []byte("id"),
			"consumer.key": []byte("secret"),
		},
	}
	_, err = k8sHelper.GetClientset().CoreV1().Secrets("eclipse-che").Create(context.TODO(), bitbucketSecret, metav1.CreateOptions{})
	assert.Nil(t, err)

	checluster := &v2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: v2.CheClusterSpec{
			GitServices: v2.CheClusterGitServices{
				GitHub: []v2.GitHubService{
					{
						SecretName: "github-scm-secret",
					},
				},
				GitLab: []v2.GitLabService{
					{
						SecretName: "gitlab-scm-secret",
						Endpoint:   "gitlab-endpoint",
					},
				},
				BitBucket: []v2.BitBucketService{
					{
						SecretName: "bitbucket-scm-secret",
						Endpoint:   "bitbucket-endpoint",
					},
				},
			},
		},
	}

	err = checluster.ValidateCreate()
	assert.Nil(t, err)

	githubSecret, err = k8sHelper.GetClientset().CoreV1().Secrets("eclipse-che").Get(context.TODO(), "github-scm-secret", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "github", githubSecret.Annotations[constants.CheEclipseOrgOAuthScmServer])
	assert.Equal(t, constants.OAuthScmConfiguration, githubSecret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, githubSecret.Labels[constants.KubernetesPartOfLabelKey])

	gitlabSecret, err = k8sHelper.GetClientset().CoreV1().Secrets("eclipse-che").Get(context.TODO(), "gitlab-scm-secret", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "gitlab", gitlabSecret.Annotations[constants.CheEclipseOrgOAuthScmServer])
	assert.Equal(t, "gitlab-endpoint", gitlabSecret.Annotations[constants.CheEclipseOrgScmServerEndpoint])
	assert.Equal(t, constants.OAuthScmConfiguration, gitlabSecret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, gitlabSecret.Labels[constants.KubernetesPartOfLabelKey])

	bitbucketSecret, err = k8sHelper.GetClientset().CoreV1().Secrets("eclipse-che").Get(context.TODO(), "bitbucket-scm-secret", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "bitbucket", bitbucketSecret.Annotations[constants.CheEclipseOrgOAuthScmServer])
	assert.Equal(t, "bitbucket-endpoint", bitbucketSecret.Annotations[constants.CheEclipseOrgScmServerEndpoint])
	assert.Equal(t, constants.OAuthScmConfiguration, bitbucketSecret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, bitbucketSecret.Labels[constants.KubernetesPartOfLabelKey])
}
