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
package imagepuller

import (
	"context"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"reflect"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sigs.k8s.io/yaml"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	routev1 "github.com/openshift/api/route/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	fakeDiscovery "k8s.io/client-go/discovery/fake"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"testing"
)

var (
	namespace                = "eclipse-che"
	csvName                  = "kubernetes-imagepuller-operator.v0.0.9"
	defaultImagePullerImages string
	valueTrue                = true
)

func TestImagePullerConfiguration(t *testing.T) {
	oldBrokerMetaDataImage := strings.Split(os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata"), ":")[0] + ":old"
	oldBrokerArtifactsImage := strings.Split(os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts"), ":")[0] + ":old"
	type testCase struct {
		name                  string
		initCR                *orgv1.CheCluster
		initObjects           []runtime.Object
		expectedCR            *orgv1.CheCluster
		expectedOperatorGroup *operatorsv1.OperatorGroup
		expectedSubscription  *operatorsv1alpha1.Subscription
		expectedImagePuller   *chev1alpha1.KubernetesImagePuller
		shouldDelete          bool
	}

	testCases := []testCase{
		{
			name:   "image puller enabled, no operatorgroup, should create an operatorgroup",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				getPackageManifest(),
			},
			expectedOperatorGroup: getOperatorGroup(),
		},
		{
			name:   "image puller enabled, operatorgroup exists, should create a subscription",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
			},
			expectedSubscription: getSubscription(),
		},
		{
			name:       "image puller enabled, subscription created, should add finalizer",
			initCR:     InitCheCRWithImagePullerEnabled(),
			expectedCR: ExpectedCheCRWithImagePullerFinalizer(),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
			},
		},
		{
			name:   "image puller enabled with finalizer but default values are empty, subscription exists, should update the CR",
			initCR: InitCheCRWithImagePullerFinalizer(),
			expectedCR: &orgv1.CheCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "CheCluster",
					APIVersion: "org.eclipse.che/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            os.Getenv("CHE_FLAVOR"),
					Namespace:       namespace,
					ResourceVersion: "1",
					Finalizers: []string{
						"kubernetesimagepullers.finalizers.che.eclipse.org",
					},
				},
				Spec: orgv1.CheClusterSpec{
					ImagePuller: orgv1.CheClusterSpecImagePuller{
						Enable: true,
						Spec: chev1alpha1.KubernetesImagePullerSpec{
							DeploymentName: "kubernetes-image-puller",
							ConfigMapName:  "k8s-image-puller",
						},
					},
					Server: orgv1.CheClusterSpecServer{
						ServerExposureStrategy: "multi-host",
					},
				},
			},
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
			},
		},
		{
			name:   "image puller enabled default values already set, subscription exists, should create a KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndDefaultValuesSet(),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, user images set, subscription exists, should create a KubernetesImagePuller with user images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("image=image_url"),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url", ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, one default image set, subscription exists, should update KubernetesImagePuller default image",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";"),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				InitImagePuller(ImagePullerOptions{SpecImages: "che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "che-workspace-plugin-broker-metadata=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata") + ";", ObjectMetaResourceVersion: "2"}),
		},
		{
			name:   "image puller enabled, one default image set, subscription exists, should update KubernetesImagePuller default images while keeping user image",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("image=image_url;che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";"),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;che-workspace-plugin-broker-metadata=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata") + ";", ObjectMetaResourceVersion: "2"}),
		},
		{
			name:   "image puller enabled, default images set, subscription exists, should update KubernetesImagePuller default images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";che-workspace-plugin-broker-artifacts=" + oldBrokerArtifactsImage + ";"),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				InitImagePuller(ImagePullerOptions{SpecImages: "che-workspace-plugin-broker-metadata=" + oldBrokerMetaDataImage + ";che-workspace-plugin-broker-artifacts=" + oldBrokerArtifactsImage + ";", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "2"}),
		},
		{
			name:   "image puller enabled, latest default images set, subscription exists, should not update KubernetesImagePuller default images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet(defaultImagePullerImages),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: defaultImagePullerImages, ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, default images not set, subscription exists, should not set KubernetesImagePuller default images",
			initCR: InitCheCRWithImagePullerEnabledAndImagesSet("image=image_url;"),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;", ObjectMetaResourceVersion: "1"}),
			},
			expectedImagePuller: InitImagePuller(ImagePullerOptions{SpecImages: "image=image_url;", ObjectMetaResourceVersion: "1"}),
		},
		{
			name:   "image puller enabled, KubernetesImagePuller created and spec in CheCluster is different, should update the KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndNewValuesSet(),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				getDefaultImagePuller(),
			},
			expectedImagePuller: &chev1alpha1.KubernetesImagePuller{
				TypeMeta: metav1.TypeMeta{Kind: "KubernetesImagePuller", APIVersion: "che.eclipse.org/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "2",
					Name:            os.Getenv("CHE_FLAVOR") + "-image-puller",
					Namespace:       namespace,
					Labels: map[string]string{
						"app":                       os.Getenv("CHE_FLAVOR"),
						"component":                 "kubernetes-image-puller",
						"app.kubernetes.io/part-of": deploy.CheEclipseOrg,
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "org.eclipse.che/v1",
							Kind:               "CheCluster",
							BlockOwnerDeletion: &valueTrue,
							Controller:         &valueTrue,
							Name:               os.Getenv("CHE_FLAVOR"),
						},
					},
				},
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					ConfigMapName:  "k8s-image-puller-trigger-update",
					DeploymentName: "kubernetes-image-puller-trigger-update",
				},
			},
		},
		{
			name:   "image puller already created, imagePuller disabled, should delete everything",
			initCR: InitCheCRWithImagePullerDisabled(),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				getClusterServiceVersion(),
				getDefaultImagePuller(),
			},
			expectedCR:   InitCheCRWithImagePullerDisabled(),
			shouldDelete: true,
		},
		{
			name:   "image puller already created, finalizer deleted",
			initCR: InitCheCRWithImagePullerFinalizerAndDeletionTimestamp(),
			initObjects: []runtime.Object{
				getPackageManifest(),
				getOperatorGroup(),
				getSubscription(),
				getClusterServiceVersion(),
				getDefaultImagePuller(),
			},
			shouldDelete: true,
			expectedCR:   nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true)))

			deployContext := deploy.GetTestDeployContext(testCase.initCR, []runtime.Object{})

			orgv1.SchemeBuilder.AddToScheme(deployContext.ClusterAPI.Scheme)
			packagesv1.AddToScheme(deployContext.ClusterAPI.Scheme)
			operatorsv1alpha1.AddToScheme(deployContext.ClusterAPI.Scheme)
			operatorsv1.AddToScheme(deployContext.ClusterAPI.Scheme)
			chev1alpha1.AddToScheme(deployContext.ClusterAPI.Scheme)
			routev1.AddToScheme(deployContext.ClusterAPI.Scheme)

			for _, obj := range testCase.initObjects {
				obj.(metav1.Object).SetResourceVersion("")
				err := deployContext.ClusterAPI.NonCachingClient.Create(context.TODO(), obj.(client.Object))
				if err != nil {
					t.Fatalf(err.Error())
				}
			}

			deployContext.ClusterAPI.DiscoveryClient.(*fakeDiscovery.FakeDiscovery).Fake.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "packages.operators.coreos.com/v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "PackageManifest",
						},
					},
				},
				{
					GroupVersion: "operators.coreos.com/v1alpha1",
					APIResources: []metav1.APIResource{
						{Kind: "OperatorGroup"},
						{Kind: "Subscription"},
						{Kind: "ClusterServiceVersion"},
					},
				},
				{
					GroupVersion: "che.eclipse.org/v1alpha1",
					APIResources: []metav1.APIResource{
						{Kind: "KubernetesImagePuller"},
					},
				},
			}

			var err error
			if testCase.shouldDelete {
				err = DeleteImagePullerOperatorAndFinalizer(deployContext)
				if err != nil {
					t.Fatalf("Error reconciling: %v", err)
				}
			} else {
				_, _, err = ReconcileImagePuller(deployContext)
				if err != nil {
					t.Fatalf("Error reconciling: %v", err)
				}
			}

			if testCase.expectedOperatorGroup != nil {
				gotOperatorGroup := &operatorsv1.OperatorGroup{}
				err := deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedOperatorGroup.Namespace, Name: testCase.expectedOperatorGroup.Name}, gotOperatorGroup)
				if err != nil {
					t.Errorf("Error getting OperatorGroup: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedOperatorGroup.Spec.TargetNamespaces, gotOperatorGroup.Spec.TargetNamespaces) {
					t.Errorf("Error expected target namespace %v but got %v", testCase.expectedOperatorGroup.Spec.TargetNamespaces, gotOperatorGroup.Spec.TargetNamespaces)
				}
			}
			if testCase.expectedSubscription != nil {
				gotSubscription := &operatorsv1alpha1.Subscription{}
				err := deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedSubscription.Namespace, Name: testCase.expectedSubscription.Name}, gotSubscription)
				if err != nil {
					t.Errorf("Error getting Subscription: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedSubscription.Spec, gotSubscription.Spec) {
					t.Errorf("Error, subscriptions differ (-want +got) %v", cmp.Diff(testCase.expectedSubscription.Spec, gotSubscription.Spec))
				}
			}
			// if expectedCR is not set, don't check it
			if testCase.expectedCR != nil && !reflect.DeepEqual(testCase.initCR, testCase.expectedCR) {
				gotCR := &orgv1.CheCluster{}
				err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: os.Getenv("CHE_FLAVOR")}, gotCR)
				if err != nil {
					t.Errorf("Error getting CheCluster: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedCR, gotCR) {
					t.Errorf("Expected CR and CR returned from API server are different (-want +got): %v", cmp.Diff(testCase.expectedCR, gotCR))
				}
			}
			if testCase.expectedImagePuller != nil {
				gotImagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedImagePuller.Namespace, Name: testCase.expectedImagePuller.Name}, gotImagePuller)
				if err != nil {
					t.Errorf("Error getting KubernetesImagePuller: %v", err)
				}

				diff := cmp.Diff(testCase.expectedImagePuller, gotImagePuller, cmpopts.IgnoreFields(chev1alpha1.KubernetesImagePullerSpec{}, "Images"))
				if diff != "" {
					t.Errorf("Expected KubernetesImagePuller and KubernetesImagePuller returned from API server differ (-want, +got): %v", diff)
				}

				expectedImages := nonEmptySplit(testCase.expectedImagePuller.Spec.Images, ";")
				if len(nonEmptySplit(testCase.expectedImagePuller.Spec.Images, ";")) != len(expectedImages) {
					t.Errorf("Expected KubernetesImagePuller returns %d images", len(expectedImages))
				}

				for _, expectedImage := range expectedImages {
					if !strings.Contains(gotImagePuller.Spec.Images, expectedImage) {
						t.Errorf("Expected KubernetesImagePuller returned image: %s, but it did not", expectedImage)
					}
				}
			}
			if testCase.shouldDelete {
				if testCase.expectedCR == nil {
					gotCR := &orgv1.CheCluster{}
					err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: os.Getenv("CHE_FLAVOR")}, gotCR)
					if !errors.IsNotFound(err) {
						t.Fatal("CR CheCluster should be removed")
					}
				}

				imagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: os.Getenv("CHE_FLAVOR") + "-image-puller"}, imagePuller)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found KubernetesImagePuller: %v", err)
				}

				clusterServiceVersion := &operatorsv1alpha1.ClusterServiceVersion{}
				err = deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: csvName}, clusterServiceVersion)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found ClusterServiceVersion: %v", err)
				}

				subscription := &operatorsv1alpha1.Subscription{}
				err = deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, subscription)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found Subscription: %v", err)
				}

				operatorGroup := &operatorsv1.OperatorGroup{}
				err = deployContext.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, operatorGroup)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found OperatorGroup: %v", err)
				}
			}
		})
	}
}

func TestEnvVars(t *testing.T) {
	type testcase struct {
		name     string
		env      map[string]string
		expected []ImageAndName
	}

	// unset RELATED_IMAGE environment variables, set them back
	// after tests complete
	matches := util.GetEnvByRegExp("^RELATED_IMAGE_.*")
	for _, match := range matches {
		if originalValue, exists := os.LookupEnv(match.Name); exists {
			os.Unsetenv(match.Name)
			defer os.Setenv(match.Name, originalValue)
		}
	}

	cases := []testcase{
		{
			name: "detect plugin broker images",
			env: map[string]string{
				"RELATED_IMAGE_che_workspace_plugin_broker_artifacts": "quay.io/eclipse/che-plugin-metadata-broker",
				"RELATED_IMAGE_che_workspace_plugin_broker_metadata":  "quay.io/eclipse/che-plugin-artifacts-broker",
			},
			expected: []ImageAndName{
				{Name: "che_workspace_plugin_broker_artifacts", Image: "quay.io/eclipse/che-plugin-metadata-broker"},
				{Name: "che_workspace_plugin_broker_metadata", Image: "quay.io/eclipse/che-plugin-artifacts-broker"},
			},
		},
		{
			name: "detect theia images",
			env: map[string]string{
				"RELATED_IMAGE_che_theia_plugin_registry_image_IBZWQYJ":                         "quay.io/eclipse/che-theia",
				"RELATED_IMAGE_che_theia_endpoint_runtime_binary_plugin_registry_image_IBZWQYJ": "quay.io/eclipse/che-theia-endpoint-runtime-binary",
			},
			expected: []ImageAndName{
				{Name: "che_theia_plugin_registry_image_IBZWQYJ", Image: "quay.io/eclipse/che-theia"},
				{Name: "che_theia_endpoint_runtime_binary_plugin_registry_image_IBZWQYJ", Image: "quay.io/eclipse/che-theia-endpoint-runtime-binary"},
			},
		},
		{
			name: "detect machine exec image",
			env: map[string]string{
				"RELATED_IMAGE_che_machine_exec_plugin_registry_image_IBZWQYJ":                  "quay.io/eclipse/che-machine-exec",
				"RELATED_IMAGE_codeready_workspaces_machineexec_plugin_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/machineexec-rhel8",
			},
			expected: []ImageAndName{
				{Name: "che_machine_exec_plugin_registry_image_IBZWQYJ", Image: "quay.io/eclipse/che-machine-exec"},
				{Name: "codeready_workspaces_machineexec_plugin_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/machineexec-rhel8"},
			},
		},
		{
			name: "detect plugin registry images",
			env: map[string]string{
				"RELATED_IMAGE_che_openshift_plugin_registry_image_IBZWQYJ":                          "index.docker.io/dirigiblelabs/dirigible-openshift",
				"RELATED_IMAGE_codeready_workspaces_plugin_openshift_plugin_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/plugin-openshift-rhel8",
			},
			expected: []ImageAndName{
				{Name: "che_openshift_plugin_registry_image_IBZWQYJ", Image: "index.docker.io/dirigiblelabs/dirigible-openshift"},
				{Name: "codeready_workspaces_plugin_openshift_plugin_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/plugin-openshift-rhel8"},
			},
		},
		{
			name: "detect devfile registry images",
			env: map[string]string{
				"RELATED_IMAGE_che_cpp_rhel7_devfile_registry_image_G4XDGNR":                       "quay.io/eclipse/che-cpp-rhel7",
				"RELATED_IMAGE_che_dotnet_2_2_devfile_registry_image_G4XDGNR":                      "quay.io/eclipse/che-dotnet-2.2",
				"RELATED_IMAGE_che_dotnet_3_1_devfile_registry_image_G4XDGNR":                      "quay.io/eclipse/che-dotnet-3.1",
				"RELATED_IMAGE_che_golang_1_14_devfile_registry_image_G4XDGNR":                     "quay.io/eclipse/che-golang-1.14",
				"RELATED_IMAGE_che_php_7_devfile_registry_image_G4XDGNR":                           "quay.io/eclipse/che-php-7",
				"RELATED_IMAGE_che_java11_maven_devfile_registry_image_G4XDGNR":                    "quay.io/eclipse/che-java11-maven",
				"RELATED_IMAGE_che_java8_maven_devfile_registry_image_G4XDGNR":                     "quay.io/eclipse/che-java8-maven",
				"RELATED_IMAGE_codeready_workspaces_stacks_cpp_devfile_registry_image_GIXDCMQK":    "registry.redhat.io/codeready-workspaces/stacks-cpp-rhel8",
				"RELATED_IMAGE_codeready_workspaces_stacks_dotnet_devfile_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/stacks-dotnet-rhel8",
				"RELATED_IMAGE_codeready_workspaces_stacks_golang_devfile_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/stacks-golang-rhel8",
				"RELATED_IMAGE_codeready_workspaces_stacks_php_devfile_registry_image_GIXDCMQK":    "registry.redhat.io/codeready-workspaces/stacks-php-rhel8",
				"RELATED_IMAGE_codeready_workspaces_plugin_java11_devfile_registry_image_GIXDCMQK": "registry.redhat.io/codeready-workspaces/plugin-java11-rhel8",
				"RELATED_IMAGE_codeready_workspaces_plugin_java8_devfile_registry_image_GIXDCMQK":  "registry.redhat.io/codeready-workspaces/plugin-java8-rhel8",
			},
			expected: []ImageAndName{
				{Name: "che_cpp_rhel7_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-cpp-rhel7"},
				{Name: "che_dotnet_2_2_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-dotnet-2.2"},
				{Name: "che_dotnet_3_1_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-dotnet-3.1"},
				{Name: "che_golang_1_14_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-golang-1.14"},
				{Name: "che_php_7_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-php-7"},
				{Name: "che_java11_maven_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-java11-maven"},
				{Name: "che_java8_maven_devfile_registry_image_G4XDGNR", Image: "quay.io/eclipse/che-java8-maven"},
				{Name: "codeready_workspaces_stacks_cpp_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-cpp-rhel8"},
				{Name: "codeready_workspaces_stacks_dotnet_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-dotnet-rhel8"},
				{Name: "codeready_workspaces_stacks_golang_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-golang-rhel8"},
				{Name: "codeready_workspaces_stacks_php_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/stacks-php-rhel8"},
				{Name: "codeready_workspaces_plugin_java11_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/plugin-java11-rhel8"},
				{Name: "codeready_workspaces_plugin_java8_devfile_registry_image_GIXDCMQK", Image: "registry.redhat.io/codeready-workspaces/plugin-java8-rhel8"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for k, v := range c.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}
			actual := GetDefaultImages()
			if d := cmp.Diff(sortImages(c.expected), sortImages(actual)); d != "" {
				t.Errorf("Error, collected images differ (-want, +got): %v", d)
			}
		})
	}
}

func sortImages(images []ImageAndName) []ImageAndName {
	imagesCopy := make([]ImageAndName, len(images))
	copy(imagesCopy, images)
	sort.Slice(imagesCopy, func(i, j int) bool {
		return imagesCopy[i].Name < imagesCopy[j].Name
	})
	return imagesCopy
}

func InitCheCRWithImagePullerEnabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:            os.Getenv("CHE_FLAVOR"),
			Namespace:       namespace,
			ResourceVersion: "0",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "org.eclipse.che/v1",
			Kind:       "CheCluster",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func InitCheCRWithImagePullerFinalizer() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
			ResourceVersion: "0",
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "org.eclipse.che/v1",
			Kind:       "CheCluster",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func InitCheCRWithImagePullerFinalizerAndDeletionTimestamp() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
			DeletionTimestamp: &metav1.Time{Time: time.Unix(1, 0)},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func ExpectedCheCRWithImagePullerFinalizer() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: "org.eclipse.che/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
			ResourceVersion: "1",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
			Server: orgv1.CheClusterSpecServer{
				ServerExposureStrategy: "multi-host",
			},
		},
	}
}

func InitCheCRWithImagePullerDisabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: "org.eclipse.che/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            os.Getenv("CHE_FLAVOR"),
			Namespace:       namespace,
			ResourceVersion: "0",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: false,
			},
		},
	}
}

func InitCheCRWithImagePullerEnabledAndDefaultValuesSet() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "org.eclipse.che/v1",
			Kind:       "CheCluster",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName: "kubernetes-image-puller",
					ConfigMapName:  "k8s-image-puller",
				},
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(false),
			},
		},
	}
}

func InitCheCRWithImagePullerEnabledAndImagesSet(images string) *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "org.eclipse.che/v1",
			Kind:       "CheCluster",
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName: "kubernetes-image-puller",
					ConfigMapName:  "k8s-image-puller",
					Images:         images,
				},
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(false),
			},
		},
	}
}

func InitCheCRWithImagePullerEnabledAndNewValuesSet() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
				Spec: chev1alpha1.KubernetesImagePullerSpec{
					DeploymentName: "kubernetes-image-puller-trigger-update",
					ConfigMapName:  "k8s-image-puller-trigger-update",
				},
			},
			Auth: orgv1.CheClusterSpecAuth{
				OpenShiftoAuth: util.NewBoolPointer(false),
			},
		},
	}
}

type ImagePullerOptions struct {
	SpecImages                string
	ObjectMetaResourceVersion string
}

func InitImagePuller(options ImagePullerOptions) *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "che.eclipse.org/v1alpha1",
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            os.Getenv("CHE_FLAVOR") + "-image-puller",
			Namespace:       namespace,
			ResourceVersion: options.ObjectMetaResourceVersion,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": deploy.CheEclipseOrg,
				"app":                       os.Getenv("CHE_FLAVOR"),
				"component":                 "kubernetes-image-puller",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "org.eclipse.che/v1",
					Kind:               "CheCluster",
					Controller:         &valueTrue,
					BlockOwnerDeletion: &valueTrue,
					Name:               os.Getenv("CHE_FLAVOR"),
				},
			},
		},
		Spec: chev1alpha1.KubernetesImagePullerSpec{
			DeploymentName: "kubernetes-image-puller",
			ConfigMapName:  "k8s-image-puller",
			Images:         options.SpecImages,
		},
	}
}

func getDefaultImagePuller() *chev1alpha1.KubernetesImagePuller {
	return &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "che.eclipse.org/v1alpha1",
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR") + "-image-puller",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": deploy.CheEclipseOrg,
				"app":                       os.Getenv("CHE_FLAVOR"),
				"component":                 "kubernetes-image-puller",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "org.eclipse.che/v1",
					Kind:               "CheCluster",
					Controller:         &valueTrue,
					BlockOwnerDeletion: &valueTrue,
					Name:               os.Getenv("CHE_FLAVOR"),
				},
			},
		},
		Spec: chev1alpha1.KubernetesImagePullerSpec{
			DeploymentName: "kubernetes-image-puller",
			ConfigMapName:  "k8s-image-puller",
			Images:         defaultImagePullerImages,
		},
	}
}

// Split string by separator without empty elems
func nonEmptySplit(lineToSplit string, separator string) []string {
	splitFn := func(c rune) bool {
		runeChar, _ := utf8.DecodeRuneInString(separator)
		return c == runeChar
	}
	return strings.FieldsFunc(lineToSplit, splitFn)
}

func getPackageManifest() *packagesv1.PackageManifest {
	return &packagesv1.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Status: packagesv1.PackageManifestStatus{
			CatalogSource:          "community-operators",
			CatalogSourceNamespace: "olm",
			DefaultChannel:         "stable",
			PackageName:            "kubernetes-imagepuller-operator",
		},
	}
}

func getOperatorGroup() *operatorsv1.OperatorGroup {
	return &operatorsv1.OperatorGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Spec: operatorsv1.OperatorGroupSpec{
			TargetNamespaces: []string{
				namespace,
			},
		},
	}
}
func getSubscription() *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          "community-operators",
			Channel:                "stable",
			CatalogSourceNamespace: "olm",
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
			Package:                "kubernetes-imagepuller-operator",
		},
	}
}

func getClusterServiceVersion() *operatorsv1alpha1.ClusterServiceVersion {
	return &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      csvName,
		},
	}
}

func init() {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile("../../config/manager/manager.yaml")
	yaml.Unmarshal(data, operator)
	if err == nil {
		for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
			os.Setenv(env.Name, env.Value)
		}
	}
	defaultImagePullerImages = "che-workspace-plugin-broker-metadata=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_metadata") +
		";che-workspace-plugin-broker-artifacts=" + os.Getenv("RELATED_IMAGE_che_workspace_plugin_broker_artifacts") + ";"
}
