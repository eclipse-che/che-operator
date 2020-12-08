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
package che

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
	identity_provider "github.com/eclipse/che-operator/pkg/deploy/identity-provider"
	"github.com/google/go-cmp/cmp"

	"github.com/eclipse/che-operator/pkg/deploy"

	console "github.com/openshift/api/console/v1"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacapi "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"testing"
)

var (
	name            = "eclipse-che"
	namespace       = "eclipse-che"
	csvName         = "kubernetes-imagepuller-operator.v0.0.4"
	packageManifest = &packagesv1.PackageManifest{
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
	operatorGroup = &operatorsv1.OperatorGroup{
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
	subscription = &operatorsv1alpha1.Subscription{
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
	wrongSubscription = &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          "community-operators",
			Channel:                "beta",
			CatalogSourceNamespace: "olm",
			InstallPlanApproval:    operatorsv1alpha1.ApprovalAutomatic,
			Package:                "kubernetes-imagepuller-operator",
		},
	}
	valueTrue          = true
	defaultImagePuller = &chev1alpha1.KubernetesImagePuller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "che.eclipse.org/v1alpha1",
			Kind:       "KubernetesImagePuller",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che-image-puller",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": name,
				"app":                       "che",
				"component":                 "kubernetes-image-puller",
			},
			ResourceVersion: "1",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "org.eclipse.che/v1",
					Kind:               "CheCluster",
					Controller:         &valueTrue,
					BlockOwnerDeletion: &valueTrue,
					Name:               "eclipse-che",
				},
			},
		},
		Spec: chev1alpha1.KubernetesImagePullerSpec{
			DeploymentName: "kubernetes-image-puller",
			ConfigMapName:  "k8s-image-puller",
		},
	}
	clusterServiceVersion = &operatorsv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      csvName,
		},
	}
)

func init() {
	operator := &appsv1.Deployment{}
	data, err := ioutil.ReadFile("../../../deploy/operator.yaml")
	yaml.Unmarshal(data, operator)
	if err == nil {
		for _, env := range operator.Spec.Template.Spec.Containers[0].Env {
			os.Setenv(env.Name, env.Value)
		}
	}
}

func TestImagePullerConfiguration(t *testing.T) {
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
				packageManifest,
			},
			expectedOperatorGroup: operatorGroup,
		},
		{
			name:   "image puller enabled, operatorgroup exists, should create a subscription",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
			},
			expectedSubscription: subscription,
		},
		{
			name:   "image puller enabled, subscription created but has changed, should update subscription, this shouldn't happen",
			initCR: InitCheCRWithImagePullerEnabled(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				wrongSubscription,
			},
			expectedSubscription: subscription,
		},
		{
			name:       "image puller enabled, subscription created, should add finalizer",
			initCR:     InitCheCRWithImagePullerEnabled(),
			expectedCR: ExpectedCheCRWithImagePullerFinalizer(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
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
					Name:            name,
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
				},
			},
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
			},
		},
		{
			name:   "image puller enabled default values already set, subscription exists, should create a KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndDefaultValuesSet(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
			},
			expectedImagePuller: defaultImagePuller,
		},
		{
			name:   "image puller enabled, KubernetesImagePuller created and spec in CheCluster is different, should update the KubernetesImagePuller",
			initCR: InitCheCRWithImagePullerEnabledAndNewValuesSet(),
			initObjects: []runtime.Object{
				packageManifest,
				operatorGroup,
				subscription,
				defaultImagePuller,
			},
			expectedImagePuller: &chev1alpha1.KubernetesImagePuller{
				TypeMeta: metav1.TypeMeta{Kind: "KubernetesImagePuller", APIVersion: "che.eclipse.org/v1alpha1"},
				ObjectMeta: metav1.ObjectMeta{
					ResourceVersion: "2",
					Name:            name + "-image-puller",
					Namespace:       namespace,
					Labels: map[string]string{
						"app":                       "che",
						"component":                 "kubernetes-image-puller",
						"app.kubernetes.io/part-of": "eclipse-che",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "org.eclipse.che/v1",
							Kind:               "CheCluster",
							BlockOwnerDeletion: &valueTrue,
							Controller:         &valueTrue,
							Name:               name,
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
				packageManifest,
				operatorGroup,
				subscription,
				clusterServiceVersion,
				defaultImagePuller,
			},
			shouldDelete: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			logf.SetLogger(zap.LoggerTo(os.Stdout, true))
			orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
			packagesv1.AddToScheme(scheme.Scheme)
			operatorsv1alpha1.AddToScheme(scheme.Scheme)
			operatorsv1.AddToScheme(scheme.Scheme)
			chev1alpha1.AddToScheme(scheme.Scheme)
			testCase.initObjects = append(testCase.initObjects, testCase.initCR)
			cli := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			nonCachedClient := fake.NewFakeClientWithScheme(scheme.Scheme, testCase.initObjects...)
			clientSet := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := clientSet.Discovery().(*fakeDiscovery.FakeDiscovery)
			fakeDiscovery.Fake.Resources = []*metav1.APIResourceList{
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

			if !ok {
				t.Error("Error creating fake discovery client")
				os.Exit(1)
			}
			r := &ReconcileChe{
				client:          cli,
				nonCachedClient: nonCachedClient,
				discoveryClient: fakeDiscovery,
				scheme:          scheme.Scheme,
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      name,
					Namespace: namespace,
				},
			}
			_, err := r.Reconcile(req)
			if err != nil {
				t.Fatalf("Error reconciling: %v", err)
			}

			if testCase.expectedOperatorGroup != nil {
				gotOperatorGroup := &operatorsv1.OperatorGroup{}
				err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedOperatorGroup.Namespace, Name: testCase.expectedOperatorGroup.Name}, gotOperatorGroup)
				if err != nil {
					t.Errorf("Error getting OperatorGroup: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedOperatorGroup.Spec.TargetNamespaces, gotOperatorGroup.Spec.TargetNamespaces) {
					t.Errorf("Error expected target namespace %v but got %v", testCase.expectedOperatorGroup.Spec.TargetNamespaces, gotOperatorGroup.Spec.TargetNamespaces)
				}
			}
			if testCase.expectedSubscription != nil {
				gotSubscription := &operatorsv1alpha1.Subscription{}
				err := r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedSubscription.Namespace, Name: testCase.expectedSubscription.Name}, gotSubscription)
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
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, gotCR)
				if err != nil {
					t.Errorf("Error getting CheCluster: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedCR, gotCR) {
					t.Errorf("Expected CR and CR returned from API server are different (-want +got): %v", cmp.Diff(testCase.expectedCR, gotCR))
				}
			}
			if testCase.expectedImagePuller != nil {
				gotImagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: testCase.expectedImagePuller.Namespace, Name: testCase.expectedImagePuller.Name}, gotImagePuller)
				if err != nil {
					t.Errorf("Error getting KubernetesImagePuller: %v", err)
				}
				if !reflect.DeepEqual(testCase.expectedImagePuller, gotImagePuller) {
					t.Errorf("Expected KubernetesImagePuller and KubernetesImagePuller returned from API server differ (-want, +got): %v", cmp.Diff(testCase.expectedImagePuller, gotImagePuller))
				}
			}
			if testCase.shouldDelete {

				imagePuller := &chev1alpha1.KubernetesImagePuller{}
				err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name + "-image-puller"}, imagePuller)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found KubernetesImagePuller: %v", err)
				}

				clusterServiceVersion := &operatorsv1alpha1.ClusterServiceVersion{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: csvName}, clusterServiceVersion)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found ClusterServiceVersion: %v", err)
				}

				subscription := &operatorsv1alpha1.Subscription{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, subscription)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found subscription: %v", err)
				}

				operatorGroup := &operatorsv1.OperatorGroup{}
				err = r.nonCachedClient.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: "kubernetes-imagepuller-operator"}, operatorGroup)
				if err == nil || !errors.IsNotFound(err) {
					t.Fatalf("Should not have found subscription: %v", err)
				}
			}
		})
	}
}

func TestCheController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &ReconcileChe{client: cl, nonCachedClient: cl, scheme: &scheme, discoveryClient: dc, tests: true}

	// get CR
	cheCR := &orgv1.CheCluster{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get devfile-registry configmap
	devfilecm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "devfile-registry", Namespace: cheCR.Namespace}, devfilecm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", devfilecm.Name, err)
	}

	// update CR and make sure Che configmap has been updated
	cheCR.Spec.Server.TlsSupport = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	// reconcile again
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get configmap
	cm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cm.Name, err)
	}

	customCm := &corev1.ConfigMap{}

	// Custom ConfigMap should be gone
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "custom", Namespace: cheCR.Namespace}, customCm)
	if !errors.IsNotFound(err) {
		t.Errorf("Custom config map should be deleted and merged with Che ConfigMap")
	}

	// Get the custom role binding that should have been created for the role we passed in
	rb := &rbacapi.RoleBinding{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che-workspace-custom", Namespace: cheCR.Namespace}, rb); err != nil {
		t.Errorf("Custom role binding %s not found: %s", rb.Name, err)
	}

	// run a few checks to make sure the operator reconciled tls routes and updated configmap
	if cm.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"] != "true" {
		t.Errorf("ConfigMap wasn't updated. Extecting true, got: %s", cm.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"])
	}
	route := &routev1.Route{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultCheFlavor(cheCR), Namespace: cheCR.Namespace}, route); err != nil {
		t.Errorf("Route %s not found: %s", cm.Name, err)
	}
	if route.Spec.TLS.Termination != "edge" {
		t.Errorf("Test failed as %s %s is not a TLS route", route.Kind, route.Name)
	}

	// update CR and make sure Che configmap has been updated
	cheCR.Spec.Auth.OpenShiftoAuth = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get configmap and check if identity provider name and workspace project name are correctly set
	cm = &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cm.Name, err)
	}
	if cm.Data["CHE_INFRA_OPENSHIFT_PROJECT"] != "" {
		t.Errorf("ConfigMap wasn't updated properly. Extecting empty string, got: '%s'", cm.Data["CHE_INFRA_OPENSHIFT_PROJECT"])
	}

	_, isOpenshiftv4, err := util.DetectOpenShift()
	if err != nil {
		logrus.Errorf("Error detecting openshift version: %v", err)
	}
	expectedIdentityProviderName := "openshift-v3"
	if isOpenshiftv4 {
		expectedIdentityProviderName = "openshift-v4"
	}

	if cm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"] != expectedIdentityProviderName {
		t.Errorf("ConfigMap wasn't updated properly. Expecting '%s', got: '%s'", expectedIdentityProviderName, cm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"])
	}

	clusterAPI := deploy.ClusterAPI{
		Client: r.client,
		Scheme: r.scheme,
	}

	deployContext := &deploy.DeployContext{
		CheCluster: cheCR,
		ClusterAPI: clusterAPI,
	}

	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: cheCR.Name, Namespace: cheCR.Namespace}, cheCR); err != nil {
		t.Errorf("Failed to get the Che custom resource %s: %s", cheCR.Name, err)
	}
	if err = identity_provider.SyncIdentityProviderItems(deployContext, "che"); err != nil {
		t.Errorf("Failed to create the items for the identity provider: %s", err)
	}
	oAuthClientName := cheCR.Spec.Auth.OAuthClientName
	oauthSecret := cheCR.Spec.Auth.OAuthSecret
	oAuthClient := &oauth.OAuthClient{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: oAuthClientName, Namespace: ""}, oAuthClient); err != nil {
		t.Errorf("Failed to Get oAuthClient %s: %s", oAuthClient.Name, err)
	}
	if oAuthClient.Secret != oauthSecret {
		t.Errorf("Secrets do not match. Expecting %s, got %s", oauthSecret, oAuthClient.Secret)
	}

	// check if a new Postgres deployment is not created when spec.Database.ExternalDB is true
	cheCR.Spec.Database.ExternalDb = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}
	postgresDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "postgres", Namespace: cheCR.Namespace}, postgresDeployment)
	err = r.client.Delete(context.TODO(), postgresDeployment)
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: "postgres", Namespace: cheCR.Namespace}, postgresDeployment)
	if err == nil {
		t.Fatalf("Deployment postgres shoud not exist")
	}

	// check of storageClassName ends up in pvc spec
	fakeStorageClassName := "fake-storage-class-name"
	cheCR.Spec.Storage.PostgresPVCStorageClassName = fakeStorageClassName
	cheCR.Spec.Database.ExternalDb = false
	if err := r.client.Update(context.TODO(), cheCR); err != nil {
		t.Fatalf("Failed to update %s CR: %s", cheCR.Name, err)
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: cheCR.Namespace}, pvc); err != nil {
		t.Fatalf("Failed to get PVC: %s", err)
	}
	if err = r.client.Delete(context.TODO(), pvc); err != nil {
		t.Fatalf("Failed to delete PVC %s: %s", pvc.Name, err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	pvc = &corev1.PersistentVolumeClaim{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: cheCR.Namespace}, pvc); err != nil {
		t.Fatalf("Failed to get PVC: %s", err)
	}
	actualStorageClassName := pvc.Spec.StorageClassName
	if len(*actualStorageClassName) != len(fakeStorageClassName) {
		t.Fatalf("Expecting %s storageClassName, got %s", fakeStorageClassName, *actualStorageClassName)
	}

	// check if oAuthClient is deleted after CR is deleted (finalizer logic)
	// since fake api does not set deletion timestamp, CR is updated in tests rather than deleted
	logrus.Info("Updating CR with deletion timestamp")
	deletionTimestamp := &metav1.Time{Time: time.Now()}
	cheCR.DeletionTimestamp = deletionTimestamp
	if err := r.client.Update(context.TODO(), cheCR); err != nil {
		t.Fatalf("Failed to update CR: %s", err)
	}
	if err := r.ReconcileFinalizer(cheCR); err != nil {
		t.Fatal("Failed to reconcile oAuthClient")
	}
	oauthClientName := cheCR.Spec.Auth.OAuthClientName
	_, err = r.GetOAuthClient(oauthClientName)
	if err == nil {
		t.Fatalf("OauthClient %s has not been deleted", oauthClientName)
	}
	logrus.Infof("Disregard the error above. OauthClient %s has been deleted", oauthClientName)
}

func TestConfiguringLabelsForRoutes(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &ReconcileChe{client: cl, nonCachedClient: cl, scheme: &scheme, discoveryClient: dc, tests: true}

	// get CR
	cheCR := &orgv1.CheCluster{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	// reconcile
	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	cheCR.Spec.Server.CheServerRoute.Labels = "route=one"
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	// reconcile again
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get route
	route := &routev1.Route{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultCheFlavor(cheCR), Namespace: cheCR.Namespace}, route); err != nil {
		t.Errorf("Route %s not found: %s", route.Name, err)
	}

	if route.ObjectMeta.Labels["route"] != "one" {
		t.Fatalf("Route '%s' does not have label '%s'", route.Name, route)
	}
}

func TestConfiguringInternalNetworkTest(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	cl, discoveryClient, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := &ReconcileChe{client: cl, nonCachedClient: cl, discoveryClient: discoveryClient, scheme: &scheme, tests: true}

	// get CR
	cheCR := &orgv1.CheCluster{}

	// get CR
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, cheCR); err != nil {
		t.Errorf("CR not found")
	}
	cheCR.Spec.Server.UseInternalClusterSVCNames = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Errorf("Failed to update CheCluster custom resource")
	}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	// reconcile to delete che route
	_, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// reconcile to create che-route
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	clusterAPI := deploy.ClusterAPI{
		Client: r.client,
		Scheme: r.scheme,
	}
	deployContext := &deploy.DeployContext{
		CheCluster: cheCR,
		ClusterAPI: clusterAPI,
	}

	// Set up che host for route
	cheRoute, _ := deploy.GetSpecRoute(deployContext, "che", "che-host", "che-host", 8080, "")
	cl.Update(context.TODO(), cheRoute)

	// reconsile to update Che route
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// Set up keycloak host for route
	keycloakRoute, _ := deploy.GetSpecRoute(deployContext, "keycloak", "keycloak", "keycloak", 8080, "")
	cl.Update(context.TODO(), keycloakRoute)

	// Set up devfile registry host for route
	devfileRegistryRoute, _ := deploy.GetSpecRoute(deployContext, "devfile-registry", "devfile-registry", "devfile-registry", 8080, "")
	cl.Update(context.TODO(), devfileRegistryRoute)

	// Set up plugin registry host for route
	pluginRegistryRoute, _ := deploy.GetSpecRoute(deployContext, "plugin-registry", "plugin-registry", "plugin-registry", 8080, "")
	cl.Update(context.TODO(), pluginRegistryRoute)

	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	cheCm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cheCm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cheCm.Name, err)
	}

	cheAPIInternal := cheCm.Data["CHE_API_INTERNAL"]
	cheAPIInternalExpected := "http://che-host.eclipse-che.svc:8080/api"
	if cheAPIInternal != cheAPIInternalExpected {
		t.Fatalf("Che API internal url must be %s", cheAPIInternalExpected)
	}

	pluginRegistryInternal := cheCm.Data["CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL"]
	pluginRegistryInternalExpected := "http://plugin-registry.eclipse-che.svc:8080/v3"
	if pluginRegistryInternal != pluginRegistryInternalExpected {
		t.Fatalf("Plugin registry internal url must be %s", pluginRegistryInternalExpected)
	}

	devRegistryInternal := cheCm.Data["CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL"]
	devRegistryInternalExpected := "http://devfile-registry.eclipse-che.svc:8080"
	if devRegistryInternal != devRegistryInternalExpected {
		t.Fatalf("Devfile registry internal url must be %s", pluginRegistryInternalExpected)
	}

	keycloakInternal := cheCm.Data["CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL"]
	keycloakInternalExpected := "http://keycloak.eclipse-che.svc:8080/auth"
	if keycloakInternal != keycloakInternalExpected {
		t.Fatalf("Keycloak registry internal url must be %s", keycloakInternalExpected)
	}

	// update CR and make sure Che configmap has been updated
	cheCR.Spec.Server.UseInternalClusterSVCNames = false
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	_, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	cheCmWithDisabledInternalClusterSVCNames := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cheCmWithDisabledInternalClusterSVCNames); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cheCm.Name, err)
	}

	cheAPIInternal = cheCmWithDisabledInternalClusterSVCNames.Data["CHE_API_INTERNAL"]
	cheAPIInternalExpected = "http://che-host/api"
	if cheAPIInternal != cheAPIInternalExpected {
		t.Fatalf("Che API internal url must be %s", cheAPIInternalExpected)
	}

	pluginRegistryInternal = cheCmWithDisabledInternalClusterSVCNames.Data["CHE_WORKSPACE_PLUGIN__REGISTRY__INTERNAL__URL"]
	pluginRegistryInternalExpected = "http://plugin-registry/v3"
	if pluginRegistryInternal != pluginRegistryInternalExpected {
		t.Fatalf("Plugin registry internal url must be %s", pluginRegistryInternalExpected)
	}

	devRegistryInternal = cheCmWithDisabledInternalClusterSVCNames.Data["CHE_WORKSPACE_DEVFILE__REGISTRY__INTERNAL__URL"]
	devRegistryInternalExpected = "http://devfile-registry"
	if devRegistryInternal != devRegistryInternalExpected {
		t.Fatalf("Plugin registry internal url must be %s", pluginRegistryInternalExpected)
	}

	keycloakInternal = cheCmWithDisabledInternalClusterSVCNames.Data["CHE_KEYCLOAK_AUTH__INTERNAL__SERVER__URL"]
	keycloakInternalExpected = "http://keycloak/auth"
	if keycloakInternal != keycloakInternalExpected {
		t.Fatalf("Keycloak internal url must be %s", keycloakInternalExpected)
	}
}

func Init() (client.Client, discovery.DiscoveryInterface, runtime.Scheme) {
	pgPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-pg-pod",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"component": "postgres",
			},
		},
	}

	// A CheCluster custom resource with metadata and spec
	cheCR := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			// todo add some spec to check controller ifs like external db, ssl etc
			Server: orgv1.CheClusterSpecServer{
				CheWorkspaceClusterRole: "cluster-admin",
			},
		},
	}

	userList := &userv1.UserList{
		Items: []userv1.User{
			userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user1",
				},
			},
			userv1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name: "user2",
				},
			},
		},
	}

	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.DefaultCheFlavor(cheCR),
			Namespace: namespace,
		},
	}

	packageManifest := &packagesv1.PackageManifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubernetes-imagepuller-operator",
			Namespace: namespace,
		},
	}

	// Objects to track in the fake client.
	objs := []runtime.Object{
		cheCR, pgPod, userList, route, packageManifest,
	}

	oAuthClient := &oauth.OAuthClient{}
	users := &userv1.UserList{}
	user := &userv1.User{}

	// Register operator types with the runtime scheme
	scheme := scheme.Scheme
	scheme.AddKnownTypes(orgv1.SchemeGroupVersion, cheCR)
	scheme.AddKnownTypes(routev1.SchemeGroupVersion, route)
	scheme.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, users, user)
	scheme.AddKnownTypes(console.GroupVersion, &console.ConsoleLink{})
	chev1alpha1.AddToScheme(scheme)
	packagesv1.AddToScheme(scheme)
	operatorsv1.AddToScheme(scheme)
	operatorsv1alpha1.AddToScheme(scheme)

	cli := fakeclientset.NewSimpleClientset()
	fakeDiscovery, ok := cli.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		fmt.Errorf("Error creating fake discovery client")
		os.Exit(1)
	}

	// Create a fake client to mock API calls
	return fake.NewFakeClient(objs...), fakeDiscovery, *scheme
}

func InitCheCRWithImagePullerEnabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
			},
		},
	}
}

func InitCheCRWithImagePullerFinalizer() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Finalizers: []string{
				"kubernetesimagepullers.finalizers.che.eclipse.org",
			},
		},
		Spec: orgv1.CheClusterSpec{
			ImagePuller: orgv1.CheClusterSpecImagePuller{
				Enable: true,
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
			Name:      name,
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
		},
	}
}

func InitCheCRWithImagePullerDisabled() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
			Name:      name,
			Namespace: namespace,
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
		},
	}
}

func InitCheCRWithImagePullerEnabledAndNewValuesSet() *orgv1.CheCluster {
	return &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
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
		},
	}
}
