//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package v2

import (
	"context"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/clusterapi"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestValidateScmSecrets(t *testing.T) {
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
	err := clusterapi.Get().Client.Create(context.TODO(), githubSecret)
	assert.Nil(t, err)

	gitlabSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "gitlab-scm-secret",
			Annotations: map[string]string{
				constants.CheEclipseOrgScmServerEndpoint: "gitlab-endpoint-secret",
			},
		},
		Data: map[string][]byte{
			"id":     []byte("id"),
			"secret": []byte("secret"),
		},
	}
	err = clusterapi.Get().Client.Create(context.TODO(), gitlabSecret)
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
	err = clusterapi.Get().Client.Create(context.TODO(), bitbucketSecret)
	assert.Nil(t, err)

	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			GitServices: CheClusterGitServices{
				GitHub: []GitHubService{
					{
						SecretName: "github-scm-secret",
						Endpoint:   "github-endpoint",
					},
				},
				GitLab: []GitLabService{
					{
						SecretName: "gitlab-scm-secret",
						Endpoint:   "gitlab-endpoint-checluster",
					},
				},
				BitBucket: []BitBucketService{
					{
						SecretName: "bitbucket-scm-secret",
					},
				},
			},
		},
	}

	cheClusterValidator := CheClusterValidator{}

	_, err = cheClusterValidator.ValidateCreate(context.TODO(), checluster)
	assert.Nil(t, err)

	secret := &corev1.Secret{}

	key := client.ObjectKey{Namespace: "eclipse-che", Name: "github-scm-secret"}
	err = clusterapi.Get().NonCachingClient.Get(context.TODO(), key, secret)

	assert.Nil(t, err)
	assert.Equal(t, "github", secret.Annotations[constants.CheEclipseOrgOAuthScmServer])
	assert.Equal(t, "github-endpoint", secret.Annotations[constants.CheEclipseOrgScmServerEndpoint])
	assert.Equal(t, constants.OAuthScmConfiguration, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])

	key = client.ObjectKey{Namespace: "eclipse-che", Name: "gitlab-scm-secret"}
	err = clusterapi.Get().NonCachingClient.Get(context.TODO(), key, secret)

	assert.Nil(t, err)
	assert.Equal(t, "gitlab", secret.Annotations[constants.CheEclipseOrgOAuthScmServer])
	assert.Equal(t, "gitlab-endpoint-secret", secret.Annotations[constants.CheEclipseOrgScmServerEndpoint])
	assert.Equal(t, constants.OAuthScmConfiguration, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])

	key = client.ObjectKey{Namespace: "eclipse-che", Name: "bitbucket-scm-secret"}
	err = clusterapi.Get().NonCachingClient.Get(context.TODO(), key, secret)

	assert.Nil(t, err)
	assert.Equal(t, "bitbucket", secret.Annotations[constants.CheEclipseOrgOAuthScmServer])
	assert.Empty(t, secret.Annotations[constants.CheEclipseOrgScmServerEndpoint])
	assert.Equal(t, constants.OAuthScmConfiguration, secret.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
}

func TestValidateOpenVSXClaimSize(t *testing.T) {
	cheClusterValidator := CheClusterValidator{}

	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			Components: CheClusterComponents{
				OpenVSXRegistry: OpenVSXRegistry{
					Database: &OpenVSXDatabase{
						Storage: &PVC{ClaimSize: "5Gi"},
					},
					Server: &OpenVSXServer{
						Storage: &PVC{ClaimSize: "4Gi"},
					},
				},
			},
		},
	}

	err := cheClusterValidator.validate(context.TODO(), checluster)
	assert.NoError(t, err)
}

func TestValidateOpenVSXServerClaimSizeInvalid(t *testing.T) {
	cheClusterValidator := CheClusterValidator{}

	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			Components: CheClusterComponents{
				OpenVSXRegistry: OpenVSXRegistry{
					Server: &OpenVSXServer{
						Storage: &PVC{ClaimSize: "A"},
					},
				},
			},
		},
	}

	err := cheClusterValidator.validate(context.TODO(), checluster)
	assert.Error(t, err)
}

func TestValidateOpenVSXSDatabaseClaimSizeInvalid(t *testing.T) {
	cheClusterValidator := CheClusterValidator{}

	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			Components: CheClusterComponents{
				OpenVSXRegistry: OpenVSXRegistry{
					Database: &OpenVSXDatabase{
						Storage: &PVC{ClaimSize: "B"},
					},
				},
			},
		},
	}

	err := cheClusterValidator.validate(context.TODO(), checluster)
	assert.Error(t, err)
}

func TestValidateOpenVSXCredentialsSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "openvsx-credentials",
		},
		Data: map[string][]byte{
			"database-user":           []byte("user"),
			"database-password":       []byte("password"),
			"database-name":           []byte("openvsx"),
			"openvsx-publisher-name":  []byte("pub"),
			"openvsx-publisher-token": []byte("pub-token"),
			"openvsx-admin-name":      []byte("admin"),
			"openvsx-admin-token":     []byte("admin-token"),
		},
	}
	err := clusterapi.Get().Client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	cheClusterValidator := CheClusterValidator{}
	credentialsSecretName := "openvsx-credentials"
	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			Components: CheClusterComponents{
				OpenVSXRegistry: OpenVSXRegistry{
					CredentialsSecretName: &credentialsSecretName,
				},
			},
		},
	}

	err = cheClusterValidator.validate(context.TODO(), checluster)
	assert.NoError(t, err)
}

func TestValidateOpenVSXCredentialsSecretMissingKeys(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "openvsx-credentials-incomplete",
		},
		Data: map[string][]byte{
			"database-user": []byte("user"),
		},
	}
	err := clusterapi.Get().Client.Create(context.TODO(), secret)
	assert.NoError(t, err)

	cheClusterValidator := CheClusterValidator{}
	credentialsSecretName := "openvsx-credentials-incomplete"
	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			Components: CheClusterComponents{
				OpenVSXRegistry: OpenVSXRegistry{
					CredentialsSecretName: &credentialsSecretName,
				},
			},
		},
	}

	err = cheClusterValidator.validate(context.TODO(), checluster)
	assert.Error(t, err)
}

func TestValidateScmSecretsShouldThrowError(t *testing.T) {
	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: CheClusterSpec{
			Networking: CheClusterSpecNetworking{Domain: "example.com"},
			GitServices: CheClusterGitServices{
				GitHub: []GitHubService{
					{
						SecretName: "github-scm-secret-with-errors",
					},
				},
			},
		},
	}

	cheClusterValidator := CheClusterValidator{}
	_, err := cheClusterValidator.ValidateCreate(context.TODO(), checluster)
	assert.Error(t, err)
	assert.Equal(t, "secret 'github-scm-secret-with-errors' not found", err.Error())

	githubSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "github-scm-secret-with-errors",
		},
	}

	err = clusterapi.Get().Client.Create(context.TODO(), githubSecret)
	assert.Nil(t, err)

	_, err = cheClusterValidator.ValidateCreate(context.TODO(), checluster)
	assert.Error(t, err)
	assert.Equal(t, "mandatory keys [id, secret] not found in secret github-scm-secret-with-errors", err.Error())
}

func TestValidateNetworkingDomainOnKubernetes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	cheClusterValidator := CheClusterValidator{}

	checluster := &CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
	}

	err := cheClusterValidator.validate(context.TODO(), checluster)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "spec.networking.domain")

	checluster.Spec.Networking.Domain = "example.com"
	err = cheClusterValidator.validate(context.TODO(), checluster)
	assert.NoError(t, err)
}
