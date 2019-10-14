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
	"time"

	console "github.com/openshift/api/console/v1"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	oauth "github.com/openshift/api/oauth/v1"
	routev1 "github.com/openshift/api/route/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacapi "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"testing"
)

func TestCheController(t *testing.T) {

	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	var (
		name      = "eclipse-che"
		namespace = "eclipse-che"
	)

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

	// Objects to track in the fake client.
	objs := []runtime.Object{
		cheCR, pgPod, userList,
	}

	route := &routev1.Route{}
	oAuthClient := &oauth.OAuthClient{}
	users := &userv1.UserList{}
	user := &userv1.User{}

	// Register operator types with the runtime scheme
	s := scheme.Scheme
	s.AddKnownTypes(orgv1.SchemeGroupVersion, cheCR)
	s.AddKnownTypes(routev1.SchemeGroupVersion, route)
	s.AddKnownTypes(oauth.SchemeGroupVersion, oAuthClient)
	s.AddKnownTypes(userv1.SchemeGroupVersion, users, user)

	s.AddKnownTypes(console.GroupVersion, &console.ConsoleLink{})

	// Create a fake client to mock API calls
	cl := fake.NewFakeClient(objs...)
	tests := true

	// Create a ReconcileChe object with the scheme and fake client
	r := &ReconcileChe{client: cl, nonCachedClient: cl, scheme: s, tests: tests}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.Requeue {
		t.Error("Reconcile did not requeue request as expected")
	}

	// update CR and make sure Che configmap has been updated
	cheCR.Spec.Server.TlsSupport = true
	if err := cl.Update(context.TODO(), cheCR); err != nil {
		t.Error("Failed to update CheCluster custom resource")
	}

	// reconcile again
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

	// get configmap
	cm := &corev1.ConfigMap{}
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, cm); err != nil {
		t.Errorf("ConfigMap %s not found: %s", cm.Name, err)
	}

	customCm := &corev1.ConfigMap{}

	// Reconcile to delete legacy custom configmap
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}

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
	if err := cl.Get(context.TODO(), types.NamespacedName{Name: "che", Namespace: cheCR.Namespace}, route); err != nil {
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
	res, err = r.Reconcile(req)
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

	err = r.client.Get(context.TODO(), types.NamespacedName{Name: cheCR.Name, Namespace: cheCR.Namespace}, cheCR)
	err = r.CreateIdentityProviderItems(cheCR, req, "che", "keycloak", false)
	oAuthClientName := cheCR.Spec.Auth.OAuthClientName
	oauthSecret := cheCR.Spec.Auth.OAuthSecret
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
	res, err = r.Reconcile(req)
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
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: "postgres-data", Namespace: cheCR.Namespace}, pvc); err != nil {
		t.Fatalf("Failed to get PVC: %s", err)
	}
	if err = r.client.Delete(context.TODO(), pvc); err != nil {
		t.Fatalf("Failed to delete PVC %s: %s", pvc.Name, err)
	}
	res, err = r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	pvc = &corev1.PersistentVolumeClaim{}
	if err = r.client.Get(context.TODO(), types.NamespacedName{Name: "postgres-data", Namespace: cheCR.Namespace}, pvc); err != nil {
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
