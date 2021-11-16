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

package sync

import (
	"context"
	"reflect"
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	corev1.AddToScheme(scheme)
}

func TestSyncCreates(t *testing.T) {

	preexisting := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
		},
	}

	new := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "new",
			Namespace: "default",
		},
	}

	cl := fake.NewFakeClientWithScheme(scheme, preexisting)

	syncer := Syncer{client: cl, scheme: scheme}

	syncer.Sync(context.TODO(), preexisting, new, cmp.Options{})

	synced := &corev1.Pod{}
	key := client.ObjectKey{Name: "new", Namespace: "default"}

	cl.Get(context.TODO(), key, synced)

	if synced.Name != "new" {
		t.Error("The synced object should have the expected name")
	}

	if len(synced.OwnerReferences) == 0 {
		t.Fatal("There should have been an owner reference set")
	}

	if synced.OwnerReferences[0].Name != "preexisting" {
		t.Error("Unexpected owner reference")
	}
}

func TestSyncUpdates(t *testing.T) {
	preexisting := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name: "preexisting",
					Kind: "Pod",
				},
			},
		},
	}

	newOwner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "newOwner",
			Namespace: "default",
		},
	}

	update := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			Labels: map[string]string{
				"a": "b",
			},
		},
	}

	cl := fake.NewFakeClientWithScheme(scheme, preexisting)

	syncer := Syncer{client: cl, scheme: scheme}

	syncer.Sync(context.TODO(), newOwner, update, cmp.Options{})

	synced := &corev1.Pod{}
	key := client.ObjectKey{Name: "preexisting", Namespace: "default"}

	cl.Get(context.TODO(), key, synced)

	if synced.Name != "preexisting" {
		t.Error("The synced object should have the expected name")
	}

	if len(synced.OwnerReferences) == 0 {
		t.Fatal("There should have been an owner reference set")
	}

	if synced.OwnerReferences[0].Name != "newOwner" {
		t.Error("Unexpected owner reference")
	}

	if len(synced.GetLabels()) == 0 {
		t.Fatal("There should have been labels on the synced object")
	}

	if synced.GetLabels()["a"] != "b" {
		t.Error("Unexpected label")
	}
}

func TestSyncKeepsAdditionalAnnosAndLabels(t *testing.T) {
	preexisting := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			Labels: map[string]string{
				"a": "x",
				"k": "v",
			},
			Annotations: map[string]string{
				"a": "x",
				"k": "v",
			},
		},
	}

	owner := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "owner",
			Namespace: "default",
		},
	}

	update := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "preexisting",
			Namespace: "default",
			Labels: map[string]string{
				"a": "b",
				"c": "d",
			},
			Annotations: map[string]string{
				"a": "b",
				"c": "d",
			},
		},
	}

	cl := fake.NewFakeClientWithScheme(scheme, preexisting)

	syncer := Syncer{client: cl, scheme: scheme}

	syncer.Sync(context.TODO(), owner, update, cmp.Options{})

	synced := &corev1.Pod{}
	key := client.ObjectKey{Name: "preexisting", Namespace: "default"}

	cl.Get(context.TODO(), key, synced)

	if synced.Name != "preexisting" {
		t.Error("The synced object should have the expected name")
	}

	expectedValues := map[string]string{
		"a": "b",
		"k": "v",
		"c": "d",
	}

	if !reflect.DeepEqual(expectedValues, synced.Labels) {
		t.Fatal("Unexpected labels on the synced object")
	}

	if !reflect.DeepEqual(expectedValues, synced.Annotations) {
		t.Fatal("Unexpected annotations on the synced object")
	}
}
