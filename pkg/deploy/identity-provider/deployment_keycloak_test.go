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
	"context"
	"os"
	"reflect"

	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"

	"github.com/eclipse-che/che-operator/pkg/deploy"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
			memoryLimit:   deploy.DefaultIdentityProviderMemoryLimit,
			memoryRequest: deploy.DefaultIdentityProviderMemoryRequest,
			cpuLimit:      deploy.DefaultIdentityProviderCpuLimit,
			cpuRequest:    deploy.DefaultIdentityProviderCpuRequest,
			cheCluster: &orgv1.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "che-cluster",
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
					Name:      "che-cluster",
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
					Name:      "che-cluster",
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

func TestSyncKeycloakDeploymentToCluster(t *testing.T) {
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &deploy.DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: deploy.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
		Proxy: &deploy.Proxy{},
	}

	// initial sync
	done, err := SyncKeycloakDeploymentToCluster(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	actual := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.IdentityProviderName, Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	// create certs configmap
	err = cli.Create(context.TODO(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ca-certs-merged",
			Namespace: "eclipse-che",
			// Go client set up resource version 1 itself on object creation.
			// ResourceVersion: "1",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create configmap: %v", err)
	}

	// create self-signed-certificate secret
	err = cli.Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "self-signed-certificate",
			Namespace: "eclipse-che",
			// Go client set up resource version 1 itself on object creation.
			// ResourceVersion: "1",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create secret: %v", err)
	}

	caSecret := &corev1.Secret{}
	err = cli.Get(context.TODO(), types.NamespacedName{
		Name:      "self-signed-certificate",
		Namespace: "eclipse-che",
	}, caSecret)
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}
	// Let's really change something. Go client will increment secret resource version itself(from '1' to '2').
	caSecret.GenerateName = "test"

	err = cli.Update(context.TODO(), caSecret)
	if err != nil {
		t.Fatalf("Failed to update secret: %s", err.Error())
	}

	// sync a new deployment
	_, err = SyncKeycloakDeploymentToCluster(deployContext)
	if err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncKeycloakDeploymentToCluster(deployContext)
	if !done || err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	actual = &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: deploy.IdentityProviderName, Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	// check ca-certs-merged revision
	cmRevision := util.FindEnv(actual.Spec.Template.Spec.Containers[0].Env, "CM_REVISION")
	if cmRevision == nil || cmRevision.Value != "1" {
		t.Fatalf("Failed to sync deployment")
	}

	// check self-signed-certificate secret revision
	if actual.ObjectMeta.Annotations["che.self-signed-certificate.version"] != "2" {
		t.Fatalf("Failed to sync deployment")
	}
}
