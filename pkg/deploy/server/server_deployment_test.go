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
package server

import (
	"os"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-che/che-operator/pkg/deploy"

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

func TestDeployment(t *testing.T) {
	type testCase struct {
		name          string
		initObjects   []runtime.Object
		memoryLimit   string
		memoryRequest string
		cpuLimit      string
		cpuRequest    string
		cheCluster    *orgv1.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   deploy.DefaultServerMemoryLimit,
			memoryRequest: deploy.DefaultServerMemoryRequest,
			cpuLimit:      deploy.DefaultServerCpuLimit,
			cpuRequest:    deploy.DefaultServerCpuRequest,
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
		},
		{
			name:          "Test custom limits",
			initObjects:   []runtime.Object{},
			cpuLimit:      "250m",
			cpuRequest:    "150m",
			memoryLimit:   "250Mi",
			memoryRequest: "150Mi",
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: orgv1.CheClusterSpec{
					Server: orgv1.CheClusterSpecServer{
						ServerCpuLimit:      "250m",
						ServerCpuRequest:    "150m",
						ServerMemoryLimit:   "250Mi",
						ServerMemoryRequest: "150Mi",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
			}

			server := NewServer(deployContext)
			deployment, err := server.getDeploymentSpec()
			if err != nil {
				t.Fatalf("Error creating deployment: %v", err)
			}

			util.CompareResources(deployment,
				util.TestExpectedResources{
					MemoryLimit:   testCase.memoryLimit,
					MemoryRequest: testCase.memoryRequest,
					CpuRequest:    testCase.cpuRequest,
					CpuLimit:      testCase.cpuLimit,
				},
				t)

			util.ValidateSecurityContext(deployment, t)
		})
	}
}

func TestMountBitBucketOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                       string
		initObjects                []runtime.Object
		expectedConsumerKeyPathEnv corev1.EnvVar
		expectedPrivateKeyPathEnv  corev1.EnvVar
		expectedEndpointEnv        corev1.EnvVar
		expectedVolume             corev1.Volume
		expectedVolumeMount        corev1.VolumeMount
	}

	testCases := []testCase{
		{
			name: "Test",
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
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "endpoint",
						},
					},
					Data: map[string][]byte{
						"private.key":  []byte("private_key"),
						"consumer.key": []byte("consumer_key"),
					},
				},
			},
			expectedConsumerKeyPathEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH",
				Value: "/che-conf/oauth/bitbucket/consumer.key",
			},
			expectedPrivateKeyPathEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH",
				Value: "/che-conf/oauth/bitbucket/private.key",
			},
			expectedEndpointEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH1_BITBUCKET_ENDPOINT",
				Value: "endpoint",
			},
			expectedVolume: corev1.Volume{
				Name: "github-oauth-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "github-oauth-config",
					},
				},
			},
			expectedVolumeMount: corev1.VolumeMount{
				Name:      "github-oauth-config",
				MountPath: "/che-conf/oauth/bitbucket",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(nil, testCase.initObjects)

			server := NewServer(deployContext)
			deployment, err := server.getDeploymentSpec()
			assert.Nil(t, err, "Unexpected error occurred %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			env := util.FindEnv(container.Env, "CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedConsumerKeyPathEnv, *env)

			env = util.FindEnv(container.Env, "CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedPrivateKeyPathEnv, *env)

			env = util.FindEnv(container.Env, "CHE_OAUTH1_BITBUCKET_ENDPOINT")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedEndpointEnv, *env)

			volume := util.FindVolume(deployment.Spec.Template.Spec.Volumes, "github-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := util.FindVolumeMount(container.VolumeMounts, "github-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}

func TestMountGitHubOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                     string
		initObjects              []runtime.Object
		expectedIdKeyPathEnv     corev1.EnvVar
		expectedSecretKeyPathEnv corev1.EnvVar
		expectedEndpointEnv      corev1.EnvVar
		expectedVolume           corev1.Volume
		expectedVolumeMount      corev1.VolumeMount
	}

	testCases := []testCase{
		{
			name: "Test",
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
							"che.eclipse.org/oauth-scm-server":    "github",
							"che.eclipse.org/scm-server-endpoint": "endpoint",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some_id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			expectedIdKeyPathEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH",
				Value: "/che-conf/oauth/github/id",
			},
			expectedSecretKeyPathEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH",
				Value: "/che-conf/oauth/github/secret",
			},
			expectedEndpointEnv: corev1.EnvVar{
				Name:  "CHE_INTEGRATION_GITHUB_SERVER__ENDPOINTS",
				Value: "endpoint",
			},
			expectedVolume: corev1.Volume{
				Name: "github-oauth-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "github-oauth-config",
					},
				},
			},
			expectedVolumeMount: corev1.VolumeMount{
				Name:      "github-oauth-config",
				MountPath: "/che-conf/oauth/github",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(nil, testCase.initObjects)

			server := NewServer(deployContext)
			deployment, err := server.getDeploymentSpec()
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			env := util.FindEnv(container.Env, "CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedIdKeyPathEnv, *env)

			env = util.FindEnv(container.Env, "CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedSecretKeyPathEnv, *env)

			volume := util.FindVolume(deployment.Spec.Template.Spec.Volumes, "github-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := util.FindVolumeMount(container.VolumeMounts, "github-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}

func TestMountGitLabOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                     string
		initObjects              []runtime.Object
		expectedIdKeyPathEnv     corev1.EnvVar
		expectedSecretKeyPathEnv corev1.EnvVar
		expectedEndpointEnv      corev1.EnvVar
		expectedVolume           corev1.Volume
		expectedVolumeMount      corev1.VolumeMount
	}

	testCases := []testCase{
		{
			name: "Test",
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "gitlab-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "gitlab",
							"che.eclipse.org/scm-server-endpoint": "endpoint",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some_id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			expectedIdKeyPathEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH_GITLAB_CLIENTID__FILEPATH",
				Value: "/che-conf/oauth/gitlab/id",
			},
			expectedSecretKeyPathEnv: corev1.EnvVar{
				Name:  "CHE_OAUTH_GITLAB_CLIENTSECRET__FILEPATH",
				Value: "/che-conf/oauth/gitlab/secret",
			},
			expectedEndpointEnv: corev1.EnvVar{
				Name:  "CHE_INTEGRATION_GITLAB_SERVER__ENDPOINTS",
				Value: "endpoint",
			},
			expectedVolume: corev1.Volume{
				Name: "gitlab-oauth-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "gitlab-oauth-config",
					},
				},
			},
			expectedVolumeMount: corev1.VolumeMount{
				Name:      "gitlab-oauth-config",
				MountPath: "/che-conf/oauth/gitlab",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			deployContext := deploy.GetTestDeployContext(nil, testCase.initObjects)

			server := NewServer(deployContext)
			deployment, err := server.getDeploymentSpec()
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			env := util.FindEnv(container.Env, "CHE_OAUTH_GITLAB_CLIENTID__FILEPATH")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedIdKeyPathEnv, *env)

			env = util.FindEnv(container.Env, "CHE_OAUTH_GITLAB_CLIENTSECRET__FILEPATH")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedSecretKeyPathEnv, *env)

			env = util.FindEnv(container.Env, "CHE_INTEGRATION_GITLAB_SERVER__ENDPOINTS")
			assert.NotNil(t, env)
			assert.Equal(t, testCase.expectedEndpointEnv, *env)

			volume := util.FindVolume(deployment.Spec.Template.Spec.Volumes, "gitlab-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := util.FindVolumeMount(container.VolumeMounts, "gitlab-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}
