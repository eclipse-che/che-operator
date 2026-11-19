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

package server

import (
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/test/checontext"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/stretchr/testify/assert"

	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeployment(t *testing.T) {
	memoryRequest := resource.MustParse("150Mi")
	cpuRequest := resource.MustParse("150m")
	memoryLimit := resource.MustParse("250Mi")
	cpuLimit := resource.MustParse("250m")

	type testCase struct {
		name          string
		initObjects   []client.Object
		memoryLimit   string
		memoryRequest string
		cpuLimit      string
		cpuRequest    string
		cheCluster    *chev2.CheCluster
	}

	testCases := []testCase{
		{
			name:          "Test default limits",
			initObjects:   []client.Object{},
			memoryLimit:   constants.DefaultServerMemoryLimit,
			memoryRequest: constants.DefaultServerMemoryRequest,
			cpuLimit:      "0", // no CPU limit if LimitRange does not exists
			cpuRequest:    constants.DefaultServerCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
		},
		{
			name:          "Test custom limits",
			initObjects:   []client.Object{},
			cpuLimit:      "250m",
			cpuRequest:    "150m",
			memoryLimit:   "250Mi",
			memoryRequest: "150Mi",
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
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
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx := checontext.NewBuilder().WithCheCluster(testCase.cheCluster).WithObjects(testCase.initObjects...).Build()

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(cheCtx)

			assert.Nil(t, err)
			resources := deployment.Spec.Template.Spec.Containers[0].Resources
			assert.True(t, resources.Limits.Memory().Equal(resource.MustParse(testCase.memoryLimit)), "memory limit")
			assert.True(t, resources.Requests.Memory().Equal(resource.MustParse(testCase.memoryRequest)), "memory request")
			assert.True(t, resources.Limits.Cpu().Equal(resource.MustParse(testCase.cpuLimit)), "CPU limit")
			assert.True(t, resources.Requests.Cpu().Equal(resource.MustParse(testCase.cpuRequest)), "CPU request")

			securityContext := deployment.Spec.Template.Spec.Containers[0].SecurityContext
			assert.Equal(t, corev1.Capability("ALL"), securityContext.Capabilities.Drop[0])
			assert.Equal(t, ptr.To(false), securityContext.AllowPrivilegeEscalation)
		})
	}
}

func TestMountBitBucketServerOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []client.Object
		expectedConsumerKeyPath string
		expectedPrivateKeyPath  string
		expectedOAuthEndpoint   string
		expectedVolume          corev1.Volume
		expectedVolumeMount     corev1.VolumeMount
	}

	testCases := []testCase{
		{
			name: "Test",
			initObjects: []client.Object{
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
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx := checontext.NewBuilder().WithObjects(testCase.initObjects...).Build()

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(cheCtx)
			assert.Nil(t, err, "Unexpected error occurred %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH", container.Env)
			assert.Equal(t, testCase.expectedConsumerKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH", container.Env)
			assert.Equal(t, testCase.expectedPrivateKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH_BITBUCKET_ENDPOINT", container.Env)
			assert.Equal(t, testCase.expectedOAuthEndpoint, value)

			volume := utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "bitbucket-oauth-config" })
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, *volume)

			volumeMount := utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "bitbucket-oauth-config" })
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, *volumeMount)
		})
	}
}

func TestMountBitbucketOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                  string
		initObjects           []client.Object
		expectedIdKeyPath     string
		expectedSecretKeyPath string
		expectedVolume        corev1.Volume
		expectedVolumeMount   corev1.VolumeMount
	}

	testCases := []testCase{
		{
			name: "Test",
			initObjects: []client.Object{
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
			test.StartOn(infrastructure.OpenShiftV4)
			cheCtx := checontext.NewBuilder().WithObjects(testCase.initObjects...).Build()

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(cheCtx)
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH2_BITBUCKET_CLIENTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_BITBUCKET_CLIENTSECRET__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedSecretKeyPath, value)

			volume := utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "bitbucket-oauth-config" })
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, *volume)

			volumeMount := utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "bitbucket-oauth-config" })
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, *volumeMount)
		})
	}
}

func TestMountGitHubOAuthEnvVar(t *testing.T) {
	secret1 := &corev1.Secret{
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
			"id":     []byte("some_id_1"),
			"secret": []byte("some_secret_1"),
		},
	}

	secret2 := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-oauth-config_2",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/part-of":   "che.eclipse.org",
				"app.kubernetes.io/component": "oauth-scm-configuration",
			},
			Annotations: map[string]string{
				"che.eclipse.org/oauth-scm-server":                       "github",
				"che.eclipse.org/scm-server-endpoint":                    "endpoint_2",
				"che.eclipse.org/scm-github-disable-subdomain-isolation": "false",
			},
		},
		Data: map[string][]byte{
			"id":     []byte("some_id_2"),
			"secret": []byte("some_secret_2"),
		},
	}

	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := checontext.NewBuilder().WithObjects(secret1).WithObjects(secret2).Build()

	server := NewCheServerReconciler()
	deployment, err := server.getDeploymentSpec(cheCtx)
	assert.Nil(t, err, "Unexpected error %v", err)

	container := &deployment.Spec.Template.Spec.Containers[0]

	assert.Equal(t, "/che-conf/oauth/github/id", utils.GetEnvByName("CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH", container.Env))
	assert.Equal(t, "/che-conf/oauth/github__2/id", utils.GetEnvByName("CHE_OAUTH2_GITHUB_CLIENTID__FILEPATH__2", container.Env))

	assert.Equal(t, "/che-conf/oauth/github/secret", utils.GetEnvByName("CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH", container.Env))
	assert.Equal(t, "/che-conf/oauth/github__2/secret", utils.GetEnvByName("CHE_OAUTH2_GITHUB_CLIENTSECRET__FILEPATH__2", container.Env))

	assert.Equal(t, "endpoint_1", utils.GetEnvByName("CHE_INTEGRATION_GITHUB_OAUTH__ENDPOINT", container.Env))
	assert.Equal(t, "endpoint_2", utils.GetEnvByName("CHE_INTEGRATION_GITHUB_OAUTH__ENDPOINT__2", container.Env))

	assert.Equal(t, "true", utils.GetEnvByName("CHE_INTEGRATION_GITHUB_DISABLE__SUBDOMAIN__ISOLATION", container.Env))
	assert.Equal(t, "false", utils.GetEnvByName("CHE_INTEGRATION_GITHUB_DISABLE__SUBDOMAIN__ISOLATION__2", container.Env))

	assert.Equal(t,
		corev1.Volume{
			Name: "github-oauth-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "github-oauth-config",
				},
			},
		},
		*utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "github-oauth-config" }))

	assert.Equal(t,
		corev1.Volume{
			Name: "github-oauth-config_2",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "github-oauth-config_2",
				},
			},
		},
		*utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "github-oauth-config_2" }))

	assert.Equal(t,
		corev1.VolumeMount{
			Name:      "github-oauth-config",
			MountPath: "/che-conf/oauth/github",
		},
		*utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "github-oauth-config" }))

	assert.Equal(t,
		corev1.VolumeMount{
			Name:      "github-oauth-config_2",
			MountPath: "/che-conf/oauth/github__2",
		},
		*utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "github-oauth-config_2" }))
}

func TestMountAzureDevOpsOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name                    string
		initObjects             []client.Object
		expectedTenantIdKeyPath string
		expectedIdKeyPath       string
		expectedSecretKeyPath   string
		expectedOAuthEndpoint   string
		expectedVolume          corev1.Volume
		expectedVolumeMount     corev1.VolumeMount
	}

	testCases := []testCase{
		{
			name: "Test",
			initObjects: []client.Object{
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
						"tenant-id": []byte("some_tenant"),
						"id":        []byte("some_id"),
						"secret":    []byte("some_secret"),
					},
				},
			},
			expectedTenantIdKeyPath: "/che-conf/oauth/azure-devops/tenant-id",
			expectedIdKeyPath:       "/che-conf/oauth/azure-devops/id",
			expectedSecretKeyPath:   "/che-conf/oauth/azure-devops/secret",
			expectedOAuthEndpoint:   "endpoint_1",
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
			test.StartOn(infrastructure.OpenShiftV4)

			cheCtx := checontext.NewBuilder().WithObjects(testCase.initObjects...).Build()

			server := NewCheServerReconciler()
			deployment, err := server.getDeploymentSpec(cheCtx)
			assert.Nil(t, err, "Unexpected error %v", err)

			container := &deployment.Spec.Template.Spec.Containers[0]

			value := utils.GetEnvByName("CHE_OAUTH2_AZURE_DEVOPS_TENANTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedTenantIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_AZURE_DEVOPS_CLIENTID__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedIdKeyPath, value)

			value = utils.GetEnvByName("CHE_OAUTH2_AZURE_DEVOPS_CLIENTSECRET__FILEPATH", container.Env)
			assert.Equal(t, testCase.expectedSecretKeyPath, value)

			volume := utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "azure-devops-oauth-config" })
			assert.NotNil(t, volume)
			assert.Equal(t, testCase.expectedVolume, *volume)

			volumeMount := utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "azure-devops-oauth-config" })
			assert.NotNil(t, volumeMount)
			assert.Equal(t, testCase.expectedVolumeMount, *volumeMount)
		})
	}
}

func TestMountGitLabOAuthEnvVar(t *testing.T) {
	secret1 := &corev1.Secret{
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
				"che.eclipse.org/oauth-scm-server":                       "gitlab",
				"che.eclipse.org/scm-server-endpoint":                    "endpoint_1",
				"che.eclipse.org/scm-gitlab-disable-subdomain-isolation": "true",
			},
		},
		Data: map[string][]byte{
			"id":     []byte("some_id_1"),
			"secret": []byte("some_secret_1"),
		},
	}

	secret2 := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gitlab-oauth-config_2",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/part-of":   "che.eclipse.org",
				"app.kubernetes.io/component": "oauth-scm-configuration",
			},
			Annotations: map[string]string{
				"che.eclipse.org/oauth-scm-server":    "gitlab",
				"che.eclipse.org/scm-server-endpoint": "endpoint_2",
			},
		},
		Data: map[string][]byte{
			"id":     []byte("some_id_2"),
			"secret": []byte("some_secret_2"),
		},
	}

	test.StartOn(infrastructure.OpenShiftV4)

	cheCtx := checontext.NewBuilder().WithObjects(secret1).WithObjects(secret2).Build()

	server := NewCheServerReconciler()
	deployment, err := server.getDeploymentSpec(cheCtx)
	assert.Nil(t, err, "Unexpected error %v", err)

	container := &deployment.Spec.Template.Spec.Containers[0]

	assert.Equal(t, "/che-conf/oauth/gitlab/id", utils.GetEnvByName("CHE_OAUTH2_GITLAB_CLIENTID__FILEPATH", container.Env))
	assert.Equal(t, "/che-conf/oauth/gitlab__2/id", utils.GetEnvByName("CHE_OAUTH2_GITLAB_CLIENTID__FILEPATH__2", container.Env))

	assert.Equal(t, "/che-conf/oauth/gitlab/secret", utils.GetEnvByName("CHE_OAUTH2_GITLAB_CLIENTSECRET__FILEPATH", container.Env))
	assert.Equal(t, "/che-conf/oauth/gitlab__2/secret", utils.GetEnvByName("CHE_OAUTH2_GITLAB_CLIENTSECRET__FILEPATH__2", container.Env))

	assert.Equal(t, "endpoint_1", utils.GetEnvByName("CHE_INTEGRATION_GITLAB_OAUTH__ENDPOINT", container.Env))
	assert.Equal(t, "endpoint_2", utils.GetEnvByName("CHE_INTEGRATION_GITLAB_OAUTH__ENDPOINT__2", container.Env))

	assert.Equal(t,
		corev1.Volume{
			Name: "gitlab-oauth-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "gitlab-oauth-config",
				},
			},
		},
		*utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "gitlab-oauth-config" }))

	assert.Equal(t,
		corev1.Volume{
			Name: "gitlab-oauth-config_2",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "gitlab-oauth-config_2",
				},
			},
		},
		*utils.Find(deployment.Spec.Template.Spec.Volumes, func(v corev1.Volume) bool { return v.Name == "gitlab-oauth-config_2" }))

	assert.Equal(t,
		corev1.VolumeMount{
			Name:      "gitlab-oauth-config",
			MountPath: "/che-conf/oauth/gitlab",
		},
		*utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "gitlab-oauth-config" }))

	assert.Equal(t,
		corev1.VolumeMount{
			Name:      "gitlab-oauth-config_2",
			MountPath: "/che-conf/oauth/gitlab__2",
		},
		*utils.Find(container.VolumeMounts, func(vm corev1.VolumeMount) bool { return vm.Name == "gitlab-oauth-config_2" }))
}
