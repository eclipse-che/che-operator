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
package identity_provider

import (
	"os"
	"reflect"

	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"

	"github.com/eclipse/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
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
			memoryLimit:   deploy.DefaultIdentityProviderMemoryLimit,
			memoryRequest: deploy.DefaultIdentityProviderMemoryRequest,
			cpuLimit:      deploy.DefaultIdentityProviderCpuLimit,
			cpuRequest:    deploy.DefaultIdentityProviderCpuRequest,
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
					Auth: orgv1.CheClusterSpecAuth{
						IdentityProviderContainerResources: orgv1.ResourcesCustomSettings{
							Limits: orgv1.Resources{
								Cpu:    "250m",
								Memory: "250Mi",
							},
							Requests: orgv1.Resources{
								Cpu:    "150m",
								Memory: "150Mi",
							},
						},
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
				Proxy: &deploy.Proxy{},
			}

			deployment, err := GetSpecKeycloakDeployment(deployContext, nil)
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

func TestMountGitHubOAuthEnvVar(t *testing.T) {
	type testCase struct {
		name              string
		initObjects       []runtime.Object
		cheCluster        *orgv1.CheCluster
		expectedIdEnv     corev1.EnvVar
		expectedSecretEnv corev1.EnvVar
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
							"che.eclipse.org/oauth-scm-server": "github",
						},
					},
					Data: map[string][]byte{
						"id":     []byte("some__id"),
						"secret": []byte("some_secret"),
					},
				},
			},
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
				},
			},
			expectedIdEnv: corev1.EnvVar{
				Name: "GITHUB_CLIENT_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "id",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "github-oauth-config",
						},
					},
				},
			},
			expectedSecretEnv: corev1.EnvVar{
				Name: "GITHUB_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "secret",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "github-oauth-config",
						},
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
				Proxy: &deploy.Proxy{},
			}

			deployment, err := GetSpecKeycloakDeployment(deployContext, nil)
			if err != nil {
				t.Fatalf("Error creating deployment: %v", err)
			}

			container := &deployment.Spec.Template.Spec.Containers[0]
			idEnv := util.FindEnv(container.Env, "GITHUB_CLIENT_ID")
			if !reflect.DeepEqual(testCase.expectedIdEnv, *idEnv) {
				t.Errorf("Expected Env and Env returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedIdEnv, idEnv))
			}

			secretEnv := util.FindEnv(container.Env, "GITHUB_SECRET")
			if !reflect.DeepEqual(testCase.expectedSecretEnv, *secretEnv) {
				t.Errorf("Expected CR and CR returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedSecretEnv, secretEnv))
			}
		})
	}
}
