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
package deploy

import (
	"context"
	"os"
	"reflect"

	k8shelper "github.com/eclipse-che/che-operator/pkg/common/k8s-helper"

	"github.com/eclipse-che/che-operator/pkg/common/test"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/google/go-cmp/cmp"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
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

var (
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{},
					},
				},
			},
		},
	}
)

func TestMountSecret(t *testing.T) {
	type testCase struct {
		name               string
		initDeployment     *appsv1.Deployment
		expectedDeployment *appsv1.Deployment
		initObjects        []runtime.Object
	}

	testCases := []testCase{
		{
			name: "Mount secret as file",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
			expectedDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "test-volume",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: "test-volume",
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "test-volume",
											MountPath: "/test-path",
										},
									},
								},
							},
						},
					},
				},
			},
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-volume",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret", // corresponds to deployment name
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "file",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
					},
					Data: map[string][]byte{
						"key": []byte("key-data"),
					},
				},
			},
		},
		{
			name: "Mount env variable",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
			expectedDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name: "ENV_A",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "a",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
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
			},
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret", // corresponds to deployment name
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs: "env",
							constants.CheEclipseOrgEnvName: "ENV_A",
						},
					},
					Data: map[string][]byte{
						"a": []byte("a-data"),
					},
				},
			},
		},
		{
			name: "Mount several env variables",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
			expectedDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name: "ENV_A",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "a",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
													},
												},
											},
										},
										{
											Name: "ENV_B",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "b",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
													},
												},
											},
										},
										{
											Name: "ENV_C",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													Key: "c",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
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
			},
			initObjects: []runtime.Object{
				&corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-secret", // corresponds to deployment name
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:          "env",
							constants.CheEclipseOrg + "/a_env-name": "ENV_A",
							constants.CheEclipseOrg + "/b_env-name": "ENV_B",
							constants.CheEclipseOrg + "/c_env-name": "ENV_C",
						},
					},
					Data: map[string][]byte{
						"b": []byte("a-data"),
						"a": []byte("b-data"),
						"c": []byte("c-data"),
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initDeployment)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &chetypes.DeployContext{
				CheCluster: &chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "eclipse-che",
					},
				},
				ClusterAPI: chetypes.ClusterAPI{
					Client:           cli,
					NonCachingClient: cli,
					Scheme:           scheme.Scheme,
				},
			}

			err := MountSecrets(testCase.initDeployment, deployContext)
			if err != nil {
				t.Fatalf("Error mounting secret: %v", err)
			}

			if !reflect.DeepEqual(testCase.expectedDeployment, testCase.initDeployment) {
				t.Errorf("Expected deployment and deployment returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedDeployment, testCase.initDeployment))
			}
		})
	}
}

func TestMountConfigMaps(t *testing.T) {
	type testCase struct {
		name               string
		initDeployment     *appsv1.Deployment
		expectedDeployment *appsv1.Deployment
		initObjects        []runtime.Object
	}

	testCases := []testCase{
		{
			name: "Mount configmap as file",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
			expectedDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes: []corev1.Volume{
								{
									Name: "test-volume",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "test-volume",
											},
										},
									},
								},
							},
							Containers: []corev1.Container{
								{
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "test-volume",
											MountPath: "/test-path",
										},
									},
								},
							},
						},
					},
				},
			},
			initObjects: []runtime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-volume",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap", // corresponds to deployment name
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:   "file",
							constants.CheEclipseOrgMountPath: "/test-path",
						},
					},
					Data: map[string]string{
						"key": "key-data",
					},
				},
			},
		},
		{
			name: "Mount env variable",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
			expectedDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name: "ENV_A",
											ValueFrom: &corev1.EnvVarSource{
												ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
													Key: "a",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
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
			},
			initObjects: []runtime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap", // corresponds to deployment name
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs: "env",
							constants.CheEclipseOrgEnvName: "ENV_A",
						},
					},
					Data: map[string]string{
						"a": "a-data",
					},
				},
			},
		},
		{
			name: "Mount several env variables",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{}},
						},
					},
				},
			},
			expectedDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "che",
					ResourceVersion: "0",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Env: []corev1.EnvVar{
										{
											Name: "ENV_A",
											ValueFrom: &corev1.EnvVarSource{
												ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
													Key: "a",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
													},
												},
											},
										},
										{
											Name: "ENV_B",
											ValueFrom: &corev1.EnvVarSource{
												ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
													Key: "b",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
													},
												},
											},
										},
										{
											Name: "ENV_C",
											ValueFrom: &corev1.EnvVarSource{
												ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
													Key: "c",
													LocalObjectReference: corev1.LocalObjectReference{
														Name: "test-envs",
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
			},
			initObjects: []runtime.Object{
				&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-envs",
						Namespace: "eclipse-che",
						Labels: map[string]string{
							constants.KubernetesPartOfLabelKey:    constants.CheEclipseOrg,
							constants.KubernetesComponentLabelKey: "che-configmap", // corresponds to deployment name
						},
						Annotations: map[string]string{
							constants.CheEclipseOrgMountAs:          "env",
							constants.CheEclipseOrg + "/a_env-name": "ENV_A",
							constants.CheEclipseOrg + "/b_env-name": "ENV_B",
							constants.CheEclipseOrg + "/c_env-name": "ENV_C",
						},
					},
					Data: map[string]string{
						"b": "a-data",
						"a": "b-data",
						"c": "c-data",
					},
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initDeployment)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &chetypes.DeployContext{
				CheCluster: &chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "eclipse-che",
					},
				},
				ClusterAPI: chetypes.ClusterAPI{
					Client:           cli,
					NonCachingClient: cli,
					Scheme:           scheme.Scheme,
				},
			}

			err := MountConfigMaps(testCase.initDeployment, deployContext)
			if err != nil {
				t.Fatalf("Error mounting configmap: %v", err)
			}

			if !reflect.DeepEqual(testCase.expectedDeployment, testCase.initDeployment) {
				t.Errorf("Expected deployment and deployment returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedDeployment, testCase.initDeployment))
			}
		})
	}
}

func TestSyncEnvVarDeploymentToCluster(t *testing.T) {
	chev2.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &chetypes.DeployContext{
		CheCluster: &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: chetypes.ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
		Proxy: &chetypes.Proxy{},
	}

	// initial sync
	done, err := SyncDeploymentSpecToCluster(deployContext, deployment, DefaultDeploymentDiffOpts)
	if !done || err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "test-name",
			Value: "test-value",
		},
	}

	// sync deployment
	_, err = SyncDeploymentSpecToCluster(deployContext, deployment, DefaultDeploymentDiffOpts)
	if err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	// sync twice to be sure update done correctly
	done, err = SyncDeploymentSpecToCluster(deployContext, deployment, DefaultDeploymentDiffOpts)
	if !done || err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	actual := &appsv1.Deployment{}
	err = cli.Get(context.TODO(), types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	// check env var
	value := utils.GetEnvByName("test-name", actual.Spec.Template.Spec.Containers[0].Env)
	if value != "test-value" {
		t.Fatalf("Failed to sync deployment")
	}
}

func TestCustomizeDeploymentShouldNotUpdateResources(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("100Mi"),
									corev1.ResourceCPU:    resource.MustParse("1"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("200Mi"),
									corev1.ResourceCPU:    resource.MustParse("2"),
								},
							},
						},
					},
				},
			},
		},
	}

	customizationDeployment := &chev2.Deployment{
		Containers: []chev2.Container{
			{
				Name: "test",
			},
		},
	}

	ctx := test.GetDeployContext(nil, []runtime.Object{})
	err := OverrideDeployment(ctx, deployment, customizationDeployment)
	assert.Nil(t, err)

	assert.Equal(t, "1", deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().String())
	assert.Equal(t, "100Mi", deployment.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().String())
	// CPU limit is not set when possible
	assert.Equal(t, "0", deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().String())
	assert.Equal(t, "200Mi", deployment.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().String())
}

func TestCustomizeDeploymentImagePullPolicy(t *testing.T) {
	type testCase struct {
		name                    string
		initDeployment          *appsv1.Deployment
		customizationDeployment *chev2.Deployment
		expectedImagePullPolicy corev1.PullPolicy
	}

	testCases := []testCase{
		{
			name: "Should use ImagePullPolicy set explicitly",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "test",
									Image:           "test/test:test",
									ImagePullPolicy: corev1.PullIfNotPresent,
								},
							},
						},
					},
				},
			},
			customizationDeployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name:            "test",
						ImagePullPolicy: corev1.PullNever,
					},
				},
			},
			expectedImagePullPolicy: corev1.PullNever,
		},
		{
			name: "Should update ImagePullPolicy to Always for next tag",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "test/test:test",
								},
							},
						},
					},
				},
			},
			customizationDeployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Image: "test/test:next",
					},
				},
			},
			expectedImagePullPolicy: corev1.PullAlways,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(nil, []runtime.Object{})
			err := OverrideDeployment(ctx, testCase.initDeployment, testCase.customizationDeployment)
			assert.Nil(t, err)

			assert.Equal(t, testCase.expectedImagePullPolicy, testCase.initDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy)
		})
	}
}

func TestCustomizeDeploymentEnvVar(t *testing.T) {
	type testCase struct {
		name                    string
		initDeployment          *appsv1.Deployment
		customizationDeployment *chev2.Deployment
		expectedEnv             []corev1.EnvVar
	}

	testCases := []testCase{
		{
			name: "Should use ImagePullPolicy set explicitly",
			initDeployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "test",
									Env: []corev1.EnvVar{
										{
											Name:  "env_1",
											Value: "value_1",
										},
										{
											Name:  "env_3",
											Value: "value_3",
										},
									},
								},
							},
						},
					},
				},
			},
			customizationDeployment: &chev2.Deployment{
				Containers: []chev2.Container{
					{
						Name: "test",
						Env: []corev1.EnvVar{
							{
								Name:  "env_1",
								Value: "value_2",
							},
							{
								Name:  "env_2",
								Value: "value_2",
							},
						},
					},
				},
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  "env_1",
					Value: "value_2",
				},
				{
					Name:  "env_3",
					Value: "value_3",
				},
				{
					Name:  "env_2",
					Value: "value_2",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			ctx := test.GetDeployContext(nil, []runtime.Object{})
			err := OverrideDeployment(ctx, testCase.initDeployment, testCase.customizationDeployment)
			assert.Nil(t, err)

			assert.Equal(t, testCase.expectedEnv, testCase.initDeployment.Spec.Template.Spec.Containers[0].Env)
		})
	}
}

func TestShouldNotThrowErrorIfOverrideDeploymentSettingsIsEmpty(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
						},
					},
				},
			},
		},
	}

	overrideDeploymentSettings := &chev2.Deployment{}

	ctx := test.GetDeployContext(nil, []runtime.Object{})
	err := OverrideDeployment(ctx, deployment, overrideDeploymentSettings)
	assert.Nil(t, err)
}

func TestOverrideContainerCpuLimit(t *testing.T) {
	type testCase struct {
		name             string
		container        *corev1.Container
		overrideSettings *chev2.Container
		limitRange       *corev1.LimitRange
		expectedCpuLimit string
	}

	cpuLimit500m := resource.MustParse("500m")

	testCases := []testCase{
		{
			name: "No CPU limit, LimitRange does not exists",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("250m"),
					},
				},
			},
			overrideSettings: &chev2.Container{},
			expectedCpuLimit: "",
		},
		{
			name: "No CPU limit, LimitRange does not exists",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("250m"),
					},
				},
			},
			overrideSettings: nil,
			expectedCpuLimit: "",
		},
		{
			name: "CPU limit is set, LimitRange exists",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("250m"),
					},
				},
			},
			overrideSettings: &chev2.Container{},
			limitRange: &corev1.LimitRange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
				},
			},
			expectedCpuLimit: "250m",
		},
		{
			name: "Overridden CPU limit, LimitRange does not exists",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("250m"),
					},
				},
			},
			overrideSettings: &chev2.Container{
				Resources: &chev2.ResourceRequirements{
					Limits: &chev2.ResourceList{
						Cpu: &cpuLimit500m,
					},
				},
			},
			expectedCpuLimit: "500m",
		},
		{
			name: "Overridden CPU limit, LimitRange exists",
			container: &corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("250m"),
					},
				},
			},
			overrideSettings: &chev2.Container{
				Resources: &chev2.ResourceRequirements{
					Limits: &chev2.ResourceList{
						Cpu: &cpuLimit500m,
					},
				},
			},
			limitRange: &corev1.LimitRange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "eclipse-che",
				},
			},
			expectedCpuLimit: "500m",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			k8sHelper := k8shelper.New()

			if testCase.limitRange != nil {
				_, err := k8sHelper.GetClientset().CoreV1().LimitRanges("eclipse-che").Create(context.TODO(), testCase.limitRange, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			err := OverrideContainer("eclipse-che", testCase.container, testCase.overrideSettings)
			assert.NoError(t, err)

			if testCase.expectedCpuLimit == "" {
				assert.Empty(t, testCase.container.Resources.Limits[corev1.ResourceCPU])
			} else {
				assert.Equal(t, testCase.expectedCpuLimit, testCase.container.Resources.Limits.Cpu().String())
			}

			defer func() {
				if testCase.limitRange != nil {
					err := k8sHelper.GetClientset().CoreV1().LimitRanges("eclipse-che").Delete(context.TODO(), testCase.limitRange.Name, metav1.DeleteOptions{})
					assert.NoError(t, err)
				}
			}()
		})
	}
}
