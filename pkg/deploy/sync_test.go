//
// Copyright (c) 2019-2025 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"

	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	ctx := test.NewCtxBuilder().Build()

	err := ctx.ClusterAPI.Client.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	actual := &corev1.Secret{}
	exists, err := Get(ctx, testKey, actual)
	if !exists || err != nil {
		t.Fatalf("Failed to get object: %v", err)
	}
}

func TestCreateIgnoreIfExistsShouldReturnTrueIfObjectCreated(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := CreateIgnoreIfExists(ctx, testObj.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, done)

	actual := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), testKey, actual)
	assert.NoError(t, err)
	assert.NotNil(t, actual)
}

func TestCreateIgnoreIfExistsShouldReturnTrueIfObjectExist(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := ctx.ClusterAPI.Client.Create(context.TODO(), testObj.DeepCopy())
	assert.NoError(t, err)

	done, err := CreateIgnoreIfExists(ctx, testObj.DeepCopy())
	assert.NoError(t, err)
	assert.True(t, done)
}

func TestUpdate(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := ctx.ClusterAPI.Client.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	actual := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), testKey, actual)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	_, err = doUpdate(ctx.ClusterAPI.Client, ctx, actual, testObjLabeled.DeepCopy(), cmp.Options{})
	if err != nil {
		t.Fatalf("Failed to update object: %v", err)
	}

	err = ctx.ClusterAPI.Client.Get(context.TODO(), testKey, actual)
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

func TestShouldDeleteExistedObject(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	err := ctx.ClusterAPI.Client.Create(context.TODO(), testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to create object: %v", err)
	}

	done, err := Delete(ctx, testKey, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	if !done {
		t.Fatalf("Object hasn't been deleted")
	}

	actualObj := &corev1.Secret{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), testKey, actualObj)
	if err != nil && !errors.IsNotFound(err) {
		t.Fatalf("Failed to get object: %v", err)
	}

	if err == nil {
		t.Fatalf("Object hasn't been deleted")
	}
}

func TestShouldNotDeleteObject(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	done, err := Delete(ctx, testKey, testObj.DeepCopy())
	if err != nil {
		t.Fatalf("Failed to delete object: %v", err)
	}

	if !done {
		t.Fatalf("Object has not been deleted")
	}
}
