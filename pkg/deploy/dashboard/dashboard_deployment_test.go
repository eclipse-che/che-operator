//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/resource"
	fakeDiscovery "k8s.io/client-go/discovery/fake"

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
	memoryRequest := resource.MustParse("150Mi")
	cpuRequest := resource.MustParse("150m")
	memoryLimit := resource.MustParse("250Mi")
	cpuLimit := resource.MustParse("250m")

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
			cpuLimit:      "0", // CPU limit is not set when possible
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
	defaultExternalDevfileRegistriesEnvVar := os.Getenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES")
	defer func() {
		_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES", defaultExternalDevfileRegistriesEnvVar)
	}()

	_ = os.Setenv("CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES", "[{\"url\": \"https://registry.devfile.io\"}]")

	// re initialize defaults with new env var
	defaults.InitializeForTesting("../../../config/manager/manager.yaml")

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
					Name:  "CHE_DASHBOARD_INTERNAL_URL",
					Value: fmt.Sprintf("http://%s-dashboard.eclipse-che.svc:8080", defaults.GetCheFlavor()),
				},
				{
					Name:  "CHE_INTERNAL_URL",
					Value: "http://che-host.eclipse-che.svc:8080/api",
				},
				{
					Name:  "CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL",
					Value: "http://plugin-registry.eclipse-che.svc:8080/v3",
				},
				{
					Name: "OPENSHIFT_CONSOLE_URL",
				},
				{
					Name:  "CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT",
					Value: defaults.GetDashboardHeaderMessageText(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTEDITOR",
					Value: defaults.GetDevEnvironmentsDefaultEditor(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTCOMPONENTS",
					Value: defaults.GetDevEnvironmentsDefaultComponents(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES",
					Value: defaults.GetDevfileRegistryExternalDevfileRegistries(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL",
					Value: defaults.GetPluginRegistryOpenVSXURL(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DISABLECONTAINERBUILDCAPABILITIES",
					Value: defaults.GetDevEnvironmentsDisableContainerBuildCapabilities(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_CONTAINERSECURITYCONTEXT",
					Value: defaults.GetDevEnvironmentsContainerSecurityContext(),
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
					Name:  "CHE_DASHBOARD_INTERNAL_URL",
					Value: fmt.Sprintf("http://%s-dashboard.eclipse-che.svc:8080", defaults.GetCheFlavor()),
				},
				{
					Name:  "CHE_INTERNAL_URL",
					Value: "http://che-host.eclipse-che.svc:8080/api",
				},
				{
					Name:  "CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL",
					Value: "http://plugin-registry.eclipse-che.svc:8080/v3",
				},
				{
					Name:  "CHE_DEFAULT_SPEC_COMPONENTS_DEVFILEREGISTRY_EXTERNAL_DEVFILE_REGISTRIES",
					Value: defaults.GetDevfileRegistryExternalDevfileRegistries(),
				},
				{
					Name:  "OPENSHIFT_CONSOLE_URL",
					Value: "https://console-openshift-console.apps.my-host/",
				},
				{
					Name:  "CHE_DEFAULT_SPEC_COMPONENTS_DASHBOARD_HEADERMESSAGE_TEXT",
					Value: defaults.GetDashboardHeaderMessageText(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTEDITOR",
					Value: defaults.GetDevEnvironmentsDefaultEditor(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DEFAULTCOMPONENTS",
					Value: defaults.GetDevEnvironmentsDefaultComponents(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_COMPONENTS_PLUGINREGISTRY_OPENVSXURL",
					Value: defaults.GetPluginRegistryOpenVSXURL(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_DISABLECONTAINERBUILDCAPABILITIES",
					Value: defaults.GetDevEnvironmentsDisableContainerBuildCapabilities(),
				},
				{
					Name:  "CHE_DEFAULT_SPEC_DEVENVIRONMENTS_CONTAINERSECURITYCONTEXT",
					Value: defaults.GetDevEnvironmentsContainerSecurityContext(),
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
			ctx.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
				{
					APIResources: []metav1.APIResource{
						{Name: ConsoleLinksResourceName},
					},
				},
			}

			dashboard := NewDashboardReconciler()
			deployment, err := dashboard.getDashboardDeploymentSpec(ctx)

			assert.Nil(t, err)
			assert.Equal(t, len(deployment.Spec.Template.Spec.Containers), 1)
			test.AssertEqualEnvVars(t, testCase.envVars, deployment.Spec.Template.Spec.Containers[0].Env)
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
