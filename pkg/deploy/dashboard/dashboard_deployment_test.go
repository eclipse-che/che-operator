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

package dashboard

import (
	"os"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"testing"
)

func TestDashboardDeploymentSecurityContext(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	dashboard := NewDashboardReconciler()
	deployment, err := dashboard.getDashboardDeploymentSpec(ctx)

	assert.Nil(t, err)
	test.ValidateSecurityContext(deployment, t)
}

func TestDashboardDeploymentResources(t *testing.T) {
	type resourcesTestCase struct {
		name          string
		initObjects   []runtime.Object
		memoryLimit   string
		memoryRequest string
		cpuRequest    string
		cpuLimit      string
		cheCluster    *chev2.CheCluster
	}

	testCases := []resourcesTestCase{
		{
			name:          "Test default limits",
			initObjects:   []runtime.Object{},
			memoryLimit:   constants.DefaultDashboardMemoryLimit,
			memoryRequest: constants.DefaultDashboardMemoryRequest,
			cpuLimit:      constants.DefaultDashboardCpuLimit,
			cpuRequest:    constants.DefaultDashboardCpuRequest,
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
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
					Name:      "eclipse-che",
				},
				Spec: chev2.CheClusterSpec{
					Components: chev2.CheClusterComponents{
						Dashboard: chev2.Dashboard{
							Deployment: &chev2.Deployment{
								Containers: []chev2.Container{
									{
										Name: defaults.GetCheFlavor() + "-dashboard",
										Resources: &chev2.ResourceRequirements{
											Requests: &chev2.ResourceList{
												Memory: resource.MustParse("150Mi"),
												Cpu:    resource.MustParse("150m"),
											},
											Limits: &chev2.ResourceList{
												Memory: resource.MustParse("250Mi"),
												Cpu:    resource.MustParse("250m"),
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
			ctx := test.GetDeployContext(testCase.cheCluster, []runtime.Object{})

			dashboard := NewDashboardReconciler()
			deployment, err := dashboard.getDashboardDeploymentSpec(ctx)
			assert.Nil(t, err)
			test.CompareResources(deployment,
				test.TestExpectedResources{
					MemoryLimit:   testCase.memoryLimit,
					MemoryRequest: testCase.memoryRequest,
					CpuRequest:    testCase.cpuRequest,
					CpuLimit:      testCase.cpuLimit,
				},
				t)
		})
	}
}

func TestDashboardDeploymentEnvVars(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	type resourcesTestCase struct {
		name        string
		initObjects []runtime.Object
		envVars     []corev1.EnvVar
		cheCluster  *chev2.CheCluster
	}
	testCases := []resourcesTestCase{
		{
			name:        "Test provisioning Che URLs",
			initObjects: []runtime.Object{},
			envVars: []corev1.EnvVar{
				{
					Name:  "CHE_HOST",
					Value: "https://che-host",
				},
				{
					Name:  "CHE_URL",
					Value: "https://che-host",
				},
				{
					Name:  "CHECLUSTER_CR_NAMESPACE",
					Value: "eclipse-che",
				},
				{
					Name:  "CHECLUSTER_CR_NAME",
					Value: "eclipse-che",
				},
				{
					Name:  "CHE_INTERNAL_URL",
					Value: "http://che-host.eclipse-che.svc:8080/api",
				},
				{
					Name: "OPENSHIFT_CONSOLE_URL",
				},
			},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
		},
		{
			name: "Test provisioning OpenShift Console URL",
			initObjects: []runtime.Object{
				&configv1.Console{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster",
						Namespace: "openshift-console",
					},
					Status: configv1.ConsoleStatus{
						ConsoleURL: "https://console-openshift-console.apps.my-host/",
					},
				},
			},
			envVars: []corev1.EnvVar{
				{
					Name:  "CHE_HOST",
					Value: "https://che-host",
				},
				{
					Name:  "CHE_URL",
					Value: "https://che-host",
				},
				{
					Name:  "CHECLUSTER_CR_NAMESPACE",
					Value: "eclipse-che",
				},
				{
					Name:  "CHECLUSTER_CR_NAME",
					Value: "eclipse-che",
				},
				{
					Name:  "CHE_INTERNAL_URL",
					Value: "http://che-host.eclipse-che.svc:8080/api",
				},
				{
					Name:  "OPENSHIFT_CONSOLE_URL",
					Value: "https://console-openshift-console.apps.my-host/",
				},
			},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://che-host",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			ctx := test.GetDeployContext(testCase.cheCluster, testCase.initObjects)

			dashboard := NewDashboardReconciler()
			deployment, err := dashboard.getDashboardDeploymentSpec(ctx)

			assert.Nil(t, err)
			assert.Equal(t, len(deployment.Spec.Template.Spec.Containers), 1)
			assert.Empty(t, cmp.Diff(testCase.envVars, deployment.Spec.Template.Spec.Containers[0].Env))
		})
	}
}

func TestDashboardDeploymentVolumes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	type resourcesTestCase struct {
		name         string
		initObjects  []runtime.Object
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		cheCluster   *chev2.CheCluster
	}
	testCases := []resourcesTestCase{
		{
			name:        "Test provisioning Custom CAs only",
			initObjects: []runtime.Object{
				// no deploy.CheTLSSelfSignedCertificateSecretName is created
			},
			volumes: []corev1.Volume{
				{
					Name: "che-custom-ca",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ca-certs-merged",
							},
						},
					}},
			},
			volumeMounts: []corev1.VolumeMount{
				{Name: "che-custom-ca", MountPath: "/public-certs/custom"},
			},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
		},
		{
			name: "Test provisioning Che and Custom CAs",
			initObjects: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.DefaultSelfSignedCertificateSecretName,
						Namespace: "eclipse-che",
					},
				},
			},
			volumes: []corev1.Volume{
				{
					Name: "che-custom-ca",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "ca-certs-merged",
							},
						},
					}},
				{
					Name: "che-self-signed-ca",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "self-signed-certificate",
							Items: []corev1.KeyToPath{
								{
									Key:  "ca.crt",
									Path: "ca.crt",
								},
							},
						},
					},
				},
			},
			volumeMounts: []corev1.VolumeMount{
				{Name: "che-custom-ca", MountPath: "/public-certs/custom"},
				{Name: "che-self-signed-ca", MountPath: "/public-certs/che-self-signed"},
			},
			cheCluster: &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))
			ctx := test.GetDeployContext(testCase.cheCluster, testCase.initObjects)

			dashboard := NewDashboardReconciler()
			deployment, err := dashboard.getDashboardDeploymentSpec(ctx)

			assert.Nil(t, err)
			assert.Equal(t, len(deployment.Spec.Template.Spec.Containers), 1)
			assert.Empty(t, cmp.Diff(testCase.volumeMounts, deployment.Spec.Template.Spec.Containers[0].VolumeMounts))
		})
	}
}
