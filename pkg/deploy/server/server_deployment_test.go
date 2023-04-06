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

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/stretchr/testify/assert"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
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
	memoryRequest := resource.MustParse("150Mi")
	cpuRequest := resource.MustParse("150m")
	memoryLimit := resource.MustParse("250Mi")
	cpuLimit := resource.MustParse("250m")

	type testCase struct {
		name          string
		initObjects   []runtime.Object
		memoryLimit   string
		memoryRequest string
		cpuLimit      string
		cpuRequest    string
		cheCluster    *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   constants.DefaultServerMemoryLimit,
			memoryRequest: constants.DefaultServerMemoryRequest,
			cpuLimit:      constants.DefaultServerCpuLimit,
			cpuRequest:    constants.DefaultServerCpuRequest,
			cheCluster: &chev2.CheCluster{
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
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						CheServer: chev2.CheServer{
							Deployment: &chev2.Deployment{
								Containers: []chev2.Container{
									{
										Name: defaults.GetCheFlavor(),
										Resources: &chev2.ResourceRequirements{
											Requests: &chev2.ResourceList{
												Memory: &memoryRequest,
												Cpu:    &cpuRequest,
											},
											Limits: &chev2.ResourceList{
												Memory: &memoryLimit,
												Cpu:    &cpuLimit,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			ctx := &chetypes.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: chetypes.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
			}

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(ctx)

			assert.Nil(t, err)
			test.CompareResources(deployment,
				test.TestExpectedResources{
					MemoryLimit:   testCase.memoryLimit,
					MemoryRequest: testCase.memoryRequest,
					CpuRequest:    testCase.cpuRequest,
					CpuLimit:      testCase.cpuLimit,
				},
				t)

			test.ValidateSecurityContext(deployment, t)
		})
	}
}

func TestMountBitBucketServerOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []runtime.Object
		expectedConsumerKeyPath string
		expectedPrivateKeyPath  string
		expectedOAuthEndpoint   string
		expectedVolume          corev1.Volume
		expectedVolumeMount     corev1.VolumeMount
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
						Name:      "bitbucket-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "endpoint_1",
						},
					},
					Data: map[string][]byte{
						"private.key":  []byte("private_key"),
						"consumer.key": []byte("consumer_key"),
					},
				},
			},
			expectedConsumerKeyPath: "/che-conf/oauth/bitbucket/consumer.key",
			expectedPrivateKeyPath:  "/che-conf/oauth/bitbucket/private.key",
			expectedOAuthEndpoint:   "endpoint_1",
			expectedVolume: corev1.Volume{
				Name: "bitbucket-oauth-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "bitbucket-oauth-config",
					},
				},
			},
			expectedVolumeMount: corev1.VolumeMount{
				Name:      "bitbucket-oauth-config",
				MountPath: "/che-conf/oauth/bitbucket",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(nil, testCase.initObjects)

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(ctx)
			assert.Nil(t, err, "Unexpected error occurred %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH", container.Env)
			assert.Equal(t, testCase.expectedConsumerKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH", container.Env)
			assert.Equal(t, testCase.expectedPrivateKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH_BITBUCKET_ENDPOINT", container.Env)
			assert.Equal(t, testCase.expectedOAuthEndpoint, value)

			volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, "bitbucket-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := test.FindVolumeMount(container.VolumeMounts, "bitbucket-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}

func TestMountBitbucketOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                  string
		initObjects           []runtime.Object
		expectedIdKeyPath     string
		expectedSecretKeyPath string
		expectedOAuthEndpoint string
		expectedVolume        corev1.Volume
		expectedVolumeMount   corev1.VolumeMount
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
						Name:      "bitbucket-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server":    "bitbucket",
							"che.eclipse.org/scm-server-endpoint": "endpoint_1",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some_id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			expectedIdKeyPath:     "/che-conf/oauth/bitbucket/id",
			expectedSecretKeyPath: "/che-conf/oauth/bitbucket/secret",
			expectedVolume: corev1.Volume{
				Name: "bitbucket-oauth-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "bitbucket-oauth-config",
					},
				},
			},
			expectedVolumeMount: corev1.VolumeMount{
				Name:      "bitbucket-oauth-config",
				MountPath: "/che-conf/oauth/bitbucket",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(nil, testCase.initObjects)

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(ctx)
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH2_BITBUCKET_CLIENTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_BITBUCKET_CLIENTSECRET__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedSecretKeyPath, value)

			volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, "bitbucket-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := test.FindVolumeMount(container.VolumeMounts, "bitbucket-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}

func TestMountGitHubOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                              string
		initObjects                       []runtime.Object
		expectedIdKeyPath                 string
		expectedSecretKeyPath             string
		expectedOAuthEndpoint             string
		expectedDisableSubdomainIsolation string
		expectedVolume                    corev1.Volume
		expectedVolumeMount               corev1.VolumeMount
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
							"che.eclipse.org/oauth-scm-server":                       "github",
							"che.eclipse.org/scm-server-endpoint":                    "endpoint_1",
							"che.eclipse.org/scm-github-disable-subdomain-isolation": "true",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some_id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			expectedIdKeyPath:                 "/che-conf/oauth/github/id",
			expectedSecretKeyPath:             "/che-conf/oauth/github/secret",
			expectedOAuthEndpoint:             "endpoint_1",
			expectedDisableSubdomainIsolation: "true",
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
			ctx := test.GetDeployContext(nil, testCase.initObjects)

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(ctx)
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedSecretKeyPath, value)

			value = utils.GetEnvByName("CHE_INTEGRATION_GITHUB_OAUTH__ENDPOINT", container.Env)
			assert.Equal(t, testCase.expectedOAuthEndpoint, value)

			value = utils.GetEnvByName("CHE_INTEGRATION_GITHUB_DISABLE__SUBDOMAIN__ISOLATION", container.Env)
			assert.Equal(t, testCase.expectedDisableSubdomainIsolation, value)

			volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, "github-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := test.FindVolumeMount(container.VolumeMounts, "github-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}

func TestMountAzureDevOpsOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                  string
		initObjects           []runtime.Object
		expectedIdKeyPath     string
		expectedSecretKeyPath string
		expectedOAuthEndpoint string
		expectedVolume        corev1.Volume
		expectedVolumeMount   corev1.VolumeMount
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
						Name:      "azure-devops-oauth-config",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							"app.kubernetes.io/part-of":   "che.eclipse.org",
							"app.kubernetes.io/component": "oauth-scm-configuration",
						},
						Annotations: map[string]string{
							"che.eclipse.org/oauth-scm-server": "azure-devops",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some_id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			expectedIdKeyPath:     "/che-conf/oauth/azure-devops/id",
			expectedSecretKeyPath: "/che-conf/oauth/azure-devops/secret",
			expectedOAuthEndpoint: "endpoint_1",
			expectedVolume: corev1.Volume{
				Name: "azure-devops-oauth-config",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "azure-devops-oauth-config",
					},
				},
			},
			expectedVolumeMount: corev1.VolumeMount{
				Name:      "azure-devops-oauth-config",
				MountPath: "/che-conf/oauth/azure-devops",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := test.GetDeployContext(nil, testCase.initObjects)

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(ctx)
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH2_AZURE_DEVOPS_CLIENTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_AZURE_DEVOPS_CLIENTSECRET__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedSecretKeyPath, value)

			volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, "azure-devops-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := test.FindVolumeMount(container.VolumeMounts, "azure-devops-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}

func TestMountGitLabOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                  string
		initObjects           []runtime.Object
		expectedIdKeyPath     string
		expectedSecretKeyPath string
		expectedOAuthEndpoint string
		expectedVolume        corev1.Volume
		expectedVolumeMount   corev1.VolumeMount
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
							"che.eclipse.org/scm-server-endpoint": "endpoint_1",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some_id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			expectedIdKeyPath:     "/che-conf/oauth/gitlab/id",
			expectedSecretKeyPath: "/che-conf/oauth/gitlab/secret",
			expectedOAuthEndpoint: "endpoint_1",
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
			ctx := test.GetDeployContext(nil, testCase.initObjects)

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(ctx)
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH2_GITLAB_CLIENTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_GITLAB_CLIENTSECRET__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedSecretKeyPath, value)

			value = utils.GetEnvByName("CHE_INTEGRATION_GITLAB_OAUTH__ENDPOINT", container.Env)
			assert.Equal(t, testCase.expectedOAuthEndpoint, value)

			volume := test.FindVolume(deployment.Spec.Template.Spec.Volumes, "gitlab-oauth-config")
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, volume)

			volumeMount := test.FindVolumeMount(container.VolumeMounts, "gitlab-oauth-config")
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, volumeMount)
		})
	}
}
