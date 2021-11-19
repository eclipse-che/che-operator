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

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

var (
	testObj = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
		},
	}
	testObjLabeled = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-secret",
			Namespace:   "eclipse-che",
			Labels:      map[string]string{"a": "b"},
			Annotations: map[string]string{"d": "c"},
		},
		Data: map[string][]byte{"x": []byte("y")},
	}
	testKey  = client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"}
	diffOpts = cmp.Options{
		cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
	}
)

func TestGet(t *testing.T) {
	cli, deployContext := initDeployContext()

	err := cli.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	actual := &corev1.Secret{}
	exists, err := Get(deployContext, testKey, actual)
	if !exists || err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
}

func TestCreate(t *testing.T) {
	cli, deployContext := initDeployContext()

	done, err := Create(deployContext, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	if !done {
		t.Fatalf("Object has not been created")
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), testKey, actual)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}
}

func TestCreateIfNotExistsShouldReturnTrueIfObjectCreated(t *testing.T) {
	cli, deployContext := initDeployContext()

	done, err := CreateIfNotExists(deployContext, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	if !done {
		t.Fatalf("Object has not been created")
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), testKey, actual)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}
}

func TestCreateIfNotExistsShouldReturnFalseIfObjectExist(t *testing.T) {
	cli, deployContext := initDeployContext()

	err := cli.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	isCreated, err := CreateIfNotExists(deployContext, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	if isCreated {
		t.Fatalf("Object has been created")
	}
}

func TestUpdate(t *testing.T) {
	cli, deployContext := initDeployContext()

	err := cli.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), testKey, actual)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	_, err = Update(deployContext, actual, testObjLabeled.DeepCopy(), cmp.Options{})
	if err != nil {
		t.Fatalf("Failed to update object: %v", err)
	}

	err = cli.Get(context.TODO(), testKey, actual)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}

	if actual.Labels["a"] != "b" {
		t.Fatalf("Object hasn't been updated")
	}
}

func TestSyncAndAddFinalizer(t *testing.T) {
	cli, deployContext := initDeployContext()

	cli.Create(context.TODO(), deployContext.CheCluster)

	// Sync object
	done, err := SyncAndAddFinalizer(deployContext, testObj.DeepCopy(), cmp.Options{}, "test-finalizer")
	if !done || err != nil {
		t.Fatalf("Error syncing object: %v", err)
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), testKey, actual)
	if err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}

	if !util.ContainsString(deployContext.CheCluster.Finalizers, "test-finalizer") {
		t.Fatalf("Failed to add finalizer")
	}
}

func TestShouldDeleteExistedObject(t *testing.T) {
	cli, deployContext := initDeployContext()

	err := cli.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	done, err := Delete(deployContext, testKey, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	if !done {
		t.Fatalf("Object hasn't been deleted")
	}

	actualObj := &corev1.Secret{}
	err = cli.Get(context.TODO(), testKey, actualObj)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	if err == nil {
		t.Fatalf("Object hasn't been deleted")
	}
}

func TestShouldNotDeleteObject(t *testing.T) {
	_, deployContext := initDeployContext()

	done, err := Delete(deployContext, testKey, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	if !done {
		t.Fatalf("Object has not been deleted")
	}
}

func initDeployContext() (client.Client, *DeployContext) {
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
	}

	return cli, deployContext
}
