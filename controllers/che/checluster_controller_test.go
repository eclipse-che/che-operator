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

package che

import (
	"context"
	"os"

	"time"

	chev1alpha1 "github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"

	console "github.com/openshift/api/console/v1"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	packagesv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators/v1"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	fakeDiscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	configv1 "github.com/openshift/api/config/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"testing"
)

var (
	namespace = "eclipse-che"
)

func TestCheController(t *testing.T) {
	var err error

	util.IsOpenShift = true
	util.IsOpenShift4 = true

	cl, dc, scheme := Init()

	// Create a ReconcileChe object with the scheme and fake client
	r := NewReconciler(cl, cl, dc, &scheme, "")

	// get CR
	checluster := &orgv1.CheCluster{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, checluster)
	assert.Nil(t, err)

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
		},
	}

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	assert.True(t, util.IsObjectExists(cl, types.NamespacedName{Name: deploy.DevfileRegistryName, Namespace: checluster.Namespace}, &corev1.ConfigMap{}))
	assert.True(t, util.IsObjectExists(cl, types.NamespacedName{Name: deploy.PluginRegistryName, Namespace: checluster.Namespace}, &corev1.ConfigMap{}))

	// reade checluster
	err = cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, checluster)
	assert.Nil(t, err)

	// update CR and make sure Che configmap has been updated
	checluster.Spec.Server.TlsSupport = true
	err = cl.Update(context.TODO(), checluster)
	assert.Nil(t, err)

	// reconcile several times
	reconcileLoops := 4
	for i := 0; i < reconcileLoops; i++ {
		_, err = r.Reconcile(context.TODO(), req)
		assert.Nil(t, err)
	}

	// get configmap
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: checluster.Namespace}, cm)
	assert.Nil(t, err)
	assert.Equal(t, cm.Data["CHE_INFRA_OPENSHIFT_TLS__ENABLED"], "true")

	// Custom ConfigMap should be gone
	assert.False(t, util.IsObjectExists(cl, types.NamespacedName{Name: "custom", Namespace: checluster.Namespace}, &corev1.ConfigMap{}))

	// Get the custom role binding that should have been created for the role we passed in
	assert.True(t, util.IsObjectExists(cl, types.NamespacedName{Name: "che-workspace-custom", Namespace: checluster.Namespace}, &rbacv1.RoleBinding{}))

	route := &routev1.Route{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultCheFlavor(checluster), Namespace: checluster.Namespace}, route)
	assert.Nil(t, err)
	assert.Equal(t, route.Spec.TLS.Termination, routev1.TLSTerminationType("edge"))

	// reread checluster
	err = cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, checluster)
	assert.Nil(t, err)

	// update CR and make sure Che configmap has been updated
	checluster.Spec.Auth.OpenShiftoAuth = util.NewBoolPointer(true)
	err = cl.Update(context.TODO(), checluster)
	assert.Nil(t, err)

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	// get configmap and check if identity provider name and workspace project name are correctly set
	cm = &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: checluster.Namespace}, cm)
	assert.Nil(t, err)
	assert.Equal(t, cm.Data["CHE_INFRA_OPENSHIFT_OAUTH__IDENTITY__PROVIDER"], "openshift-v4")

	// reread checluster
	err = cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, checluster)
	assert.Nil(t, err)
	assert.True(t, util.IsObjectExists(cl, types.NamespacedName{Name: checluster.Spec.Auth.OAuthClientName}, &oauthv1.OAuthClient{}))

	// check if a new Postgres deployment is not created when spec.Database.ExternalDB is true
	checluster.Spec.Database.ExternalDb = true
	err = cl.Update(context.TODO(), checluster)
	assert.Nil(t, err)

	postgresDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.PostgresName, Namespace: checluster.Namespace}, postgresDeployment)
	assert.Nil(t, err)

	err = r.client.Delete(context.TODO(), postgresDeployment)
	assert.Nil(t, err)

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	assert.False(t, util.IsObjectExists(cl, types.NamespacedName{Name: deploy.PostgresName, Namespace: checluster.Namespace}, &appsv1.Deployment{}))

	// check of storageClassName ends up in pvc spec
	fakeStorageClassName := "fake-storage-class-name"
	checluster.Spec.Storage.PostgresPVCStorageClassName = fakeStorageClassName
	checluster.Spec.Database.ExternalDb = false
	err = r.client.Update(context.TODO(), checluster)
	assert.Nil(t, err)

	pvc := &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: checluster.Namespace}, pvc)
	assert.Nil(t, err)

	err = r.client.Delete(context.TODO(), pvc)
	assert.Nil(t, err)

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	pvc = &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deploy.DefaultPostgresVolumeClaimName, Namespace: checluster.Namespace}, pvc)
	assert.Nil(t, err)
	assert.Equal(t, fakeStorageClassName, *pvc.Spec.StorageClassName)

	// reread checluster
	err = cl.Get(context.TODO(), types.NamespacedName{Name: os.Getenv("CHE_FLAVOR"), Namespace: namespace}, checluster)
	assert.Nil(t, err)
	assert.Equal(t, "https://eclipse.org", checluster.Status.CheURL)

	// check if oAuthClient is deleted after CR is deleted (finalizer logic)
	// since fake api does not set deletion timestamp, CR is updated in tests rather than deleted
	deletionTimestamp := &metav1.Time{Time: time.Now()}
	checluster.DeletionTimestamp = deletionTimestamp
	err = r.client.Update(context.TODO(), checluster)
	assert.Nil(t, err)

	_, err = r.Reconcile(context.TODO(), req)
	assert.Nil(t, err)

	assert.False(t, util.IsObjectExists(cl, types.NamespacedName{Name: checluster.Spec.Auth.OAuthClientName}, &oauthv1.OAuthClient{}))
}

func Init() (client.Client, discovery.DiscoveryInterface, runtime.Scheme) {
	objs, ds, scheme := createAPIObjects()

	oAuthClient := &oauthv1.OAuthClient{}
	users := &userv1.UserList{}
	user := &userv1.User{}

	// Register operator types with the runtime scheme
	scheme.AddKnownTypes(oauthv1.SchemeGroupVersion, oAuthClient)
	scheme.AddKnownTypes(userv1.SchemeGroupVersion, users, user)
	scheme.AddKnownTypes(configv1.SchemeGroupVersion, &configv1.Proxy{})

	// Create a fake client to mock API calls
	return fake.NewFakeClient(objs...), ds, scheme
}

func createAPIObjects() ([]runtime.Object, discovery.DiscoveryInterface, runtime.Scheme) {
	pgPod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-pg-pod",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"component": deploy.PostgresName,
			},
		},
	}

	// A CheCluster custom resource with metadata and spec
	cheCR := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      os.Getenv("CHE_FLAVOR"),
			Namespace: namespace,
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				CheHost:                 "eclipse.org",
				CheWorkspaceClusterRole: "cluster-admin",
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
		cheCR, pgPod, route, packageManifest,
	}

	// Register operator types with the runtime scheme
	scheme := scheme.Scheme
	scheme.AddKnownTypes(orgv1.GroupVersion, cheCR)
	scheme.AddKnownTypes(routev1.SchemeGroupVersion, route)
	scheme.AddKnownTypes(console.GroupVersion, &console.ConsoleLink{})
	chev1alpha1.AddToScheme(scheme)
	packagesv1.AddToScheme(scheme)
	operatorsv1.AddToScheme(scheme)
	operatorsv1alpha1.AddToScheme(scheme)

	cli := fakeclientset.NewSimpleClientset()
	fakeDiscovery, ok := cli.Discovery().(*fakeDiscovery.FakeDiscovery)
	if !ok {
		logrus.Error("Error creating fake discovery client")
		os.Exit(1)
	}

	// Create a fake client to mock API calls
	return objs, fakeDiscovery, *scheme
}
