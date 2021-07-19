//
// Copyright (c) 2012-2019 Red Hat, Inc.
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
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

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
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
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
		cheCluster                 *orgv1.CheCluster
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
							"che.eclipse.org/scm-server-endpoint": "bitbucket_endpoint",
						},
					},
					Data: map[string][]byte{
						"private.key":  []byte("private_key"),
						"consumer.key": []byte("consumer_key"),
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
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
				Value: "bitbucket_endpoint",
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
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &deploy.DeployContext{
				CheCluster: testCase.cheCluster,
				ClusterAPI: deploy.ClusterAPI{
					Client: cli,
					Scheme: scheme.Scheme,
				},
				Proxy: &deploy.Proxy{},
			}

			server := NewServer(deployContext)
			deployment, err := server.getDeploymentSpec()
			if err != nil {
				t.Fatalf("Error creating deployment: %v", err)
			}

			container := &deployment.Spec.Template.Spec.Containers[0]
			env := util.FindEnv(container.Env, "CHE_OAUTH1_BITBUCKET_CONSUMERKEYPATH")
			if !reflect.DeepEqual(testCase.expectedConsumerKeyPathEnv, *env) {
				t.Errorf("Expected Env and Env returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedConsumerKeyPathEnv, env))
			}

			env = util.FindEnv(container.Env, "CHE_OAUTH1_BITBUCKET_PRIVATEKEYPATH")
			if !reflect.DeepEqual(testCase.expectedPrivateKeyPathEnv, *env) {
				t.Errorf("Expected Env and Env returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedPrivateKeyPathEnv, env))
			}

			env = util.FindEnv(container.Env, "CHE_OAUTH1_BITBUCKET_ENDPOINT")
			if !reflect.DeepEqual(testCase.expectedEndpointEnv, *env) {
				t.Errorf("Expected Env and Env returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedEndpointEnv, env))
			}

			volume := util.FindVolume(deployment.Spec.Template.Spec.Volumes, "github-oauth-config")
			if !reflect.DeepEqual(testCase.expectedVolume, volume) {
				t.Errorf("Expected Volume and Volume returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedVolume, volume))
			}

			volumeMount := util.FindVolumeMount(container.VolumeMounts, "github-oauth-config")
			if !reflect.DeepEqual(testCase.expectedVolumeMount, volumeMount) {
				t.Errorf("Expected VolumeMount and VolumeMount returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedVolumeMount, volumeMount))
			}
		})
	}
}
