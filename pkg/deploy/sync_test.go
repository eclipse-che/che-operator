//
// Copyright (c) 2021 Red Hat, Inc.
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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"testing"
)

func TestGet(t *testing.T) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)

	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	err := cli.Create(context.TODO(), secret)
	if err != nil {
		t.Fatalf("Error creating object: %v", err)
	}

	actual, err := Get(deployContext,
		client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"},
		&corev1.Secret{})

	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}
}

func TestIsExists(t *testing.T) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)

	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	err := cli.Create(context.TODO(), secret)
	if err != nil {
		t.Fatalf("Error creating object: %v", err)
	}

	exists, err := IsExists(deployContext,
		client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"},
		&corev1.Secret{})

	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if !exists {
		t.Fatalf("Object not found")
	}
}

func TestCreate(t *testing.T) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)

	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	done, err := Create(deployContext,
		client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"},
		secret)

	if err != nil {
		t.Fatalf("Error creating object: %v", err)
	}

	if !done {
		t.Fatalf("Object was not created")
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}
}

func TestUpdate(t *testing.T) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
		},
	}

	newSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
			Labels:    map[string]string{"a": "b"},
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)

	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	err := cli.Create(context.TODO(), secret)
	if err != nil {
		t.Fatalf("Error creating object: %v", err)
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	_, err = Update(deployContext, actual, newSecret, cmp.Options{})

	if err != nil {
		t.Fatalf("Error updating object: %v", err)
	}

	newActual := &corev1.Secret{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"}, newActual)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}

	if newActual.Labels["a"] != "b" {
		t.Fatalf("Object was not updated properly")
	}
}

func TestSync(t *testing.T) {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
		},
	}

	newSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "eclipse-che",
			Labels:    map[string]string{"a": "b"},
		},
	}

	orgv1.SchemeBuilder.AddToScheme(scheme.Scheme)
	cli := fake.NewFakeClientWithScheme(scheme.Scheme)

	deployContext := &DeployContext{
		CheCluster: &orgv1.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "eclipse-che",
			},
		},
		ClusterAPI: ClusterAPI{
			Client:          cli,
			NonCachedClient: cli,
			Scheme:          scheme.Scheme,
		},
	}

	err := cli.Create(context.TODO(), secret)
	if err != nil {
		t.Fatalf("Error creating object: %v", err)
	}

	actual := &corev1.Secret{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"}, actual)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	_, err = Sync(deployContext, newSecret, cmp.Options{})

	if err != nil {
		t.Fatalf("Error syncing object: %v", err)
	}

	newActual := &corev1.Secret{}
	err = cli.Get(context.TODO(), client.ObjectKey{Name: "test-secret", Namespace: "eclipse-che"}, newActual)
	if err != nil {
		t.Fatalf("Error getting object: %v", err)
	}

	if actual == nil {
		t.Fatalf("Object not found")
	}

	if newActual.Labels["a"] != "b" {
		t.Fatalf("Object was not synced properly")
	}
}
