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

	"github.com/google/go-cmp/cmp"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
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
							KubernetesPartOfLabelKey:    CheEclipseOrg,
							KubernetesComponentLabelKey: "che-secret", // corresponds to deployment name
						},
						Annotations: map[string]string{
							CheEclipseOrgMountAs:   "file",
							CheEclipseOrgMountPath: "/test-path",
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
							KubernetesPartOfLabelKey:    CheEclipseOrg,
							KubernetesComponentLabelKey: "che-secret", // corresponds to deployment name
						},
						Annotations: map[string]string{
							CheEclipseOrgMountAs: "env",
							CheEclipseOrgEnvName: "ENV_A",
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
							KubernetesPartOfLabelKey:    CheEclipseOrg,
							KubernetesComponentLabelKey: "che-secret", // corresponds to deployment name
						},
						Annotations: map[string]string{
							CheEclipseOrgMountAs:          "env",
							CheEclipseOrg + "/a_env-name": "ENV_A",
							CheEclipseOrg + "/b_env-name": "ENV_B",
							CheEclipseOrg + "/c_env-name": "ENV_C",
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
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initDeployment)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &DeployContext{
				CheCluster: &orgv1.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "eclipse-che",
					},
				},
				ClusterAPI: ClusterAPI{
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
							KubernetesPartOfLabelKey:    CheEclipseOrg,
							KubernetesComponentLabelKey: "che-configmap", // corresponds to deployment name
						},
						Annotations: map[string]string{
							CheEclipseOrgMountAs:   "file",
							CheEclipseOrgMountPath: "/test-path",
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
							KubernetesPartOfLabelKey:    CheEclipseOrg,
							KubernetesComponentLabelKey: "che-configmap", // corresponds to deployment name
						},
						Annotations: map[string]string{
							CheEclipseOrgMountAs: "env",
							CheEclipseOrgEnvName: "ENV_A",
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
							KubernetesPartOfLabelKey:    CheEclipseOrg,
							KubernetesComponentLabelKey: "che-configmap", // corresponds to deployment name
						},
						Annotations: map[string]string{
							CheEclipseOrgMountAs:          "env",
							CheEclipseOrg + "/a_env-name": "ENV_A",
							CheEclipseOrg + "/b_env-name": "ENV_B",
							CheEclipseOrg + "/c_env-name": "ENV_C",
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
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initDeployment)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)

			deployContext := &DeployContext{
				CheCluster: &orgv1.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "eclipse-che",
					},
				},
				ClusterAPI: ClusterAPI{
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
	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)
	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
				Name:      "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:           cli,
			NonCachingClient: cli,
			Scheme:           scheme.Scheme,
		},
		Proxy: &Proxy{},
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
	cmRevision := util.FindEnv(actual.Spec.Template.Spec.Containers[0].Env, "test-name")
	if cmRevision == nil || cmRevision.Value != "test-value" {
		t.Fatalf("Failed to sync deployment")
	}
}
