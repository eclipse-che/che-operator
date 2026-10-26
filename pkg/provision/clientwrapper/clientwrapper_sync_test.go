//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

//go:build integration

package clientwrapper_test

import (
	"github.com/eclipse-che/che-operator/pkg/provision/clientwrapper"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cmDiffs ignores fields and defaults set by the API server on ConfigMap objects.
var cmDiffs = cmp.Options{
	cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta"),
	cmpopts.IgnoreFields(metav1.ObjectMeta{}, "UID", "ResourceVersion", "Generation", "CreationTimestamp", "ManagedFields"),
}

var _ = Describe("Sync", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.Service{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should create object when it does not exist", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "create",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		Expect(cli.Sync(env.Context, cm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())
		Expect(cm.Kind).To(Equal("ConfigMap"))
		Expect(cm.APIVersion).To(Equal("v1"))

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "create", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"key": "value"}))
	})

	It("should update object when it is not in sync", func() {
		oldCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "update",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "old",
			},
		}
		Expect(cli.Sync(env.Context, oldCm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		newCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "update",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "new",
			},
		}
		Expect(cli.Sync(env.Context, newCm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "update", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"key": "new"}))
	})

	It("should not update object when it is in sync", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "in-sync",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		Expect(cli.Sync(env.Context, cm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "in-sync", Namespace: testNamespace}, actualCm)).To(Succeed())
		resourceVersion := actualCm.ResourceVersion

		inSyncCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "in-sync",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		Expect(cli.Sync(env.Context, inSyncCm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "in-sync", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.ResourceVersion).To(Equal(resourceVersion))
	})

	It("should merge labels and annotations from the existing object", func() {
		existing := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "merge",
				Namespace: testNamespace,
				Labels: map[string]string{
					"label1": "cluster_value1",
					"label2": "cluster_value2",
				},
				Annotations: map[string]string{
					"annotation1": "cluster_value1",
					"annotation2": "cluster_value2",
				},
			},
		}
		Expect(cli.Sync(env.Context, existing, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		blueprint := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "merge",
				Namespace: testNamespace,
				Labels: map[string]string{
					"label1": "value1",
					"label3": "value3",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation3": "value3",
				},
			},
		}
		Expect(cli.Sync(env.Context, blueprint, &clientwrapper.SyncOptions{DiffOpts: cmDiffs, MergeLabels: true, MergeAnnotations: true})).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "merge", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Labels).To(Equal(map[string]string{
			"label1": "value1",
			"label2": "cluster_value2",
			"label3": "value3",
		}))
		Expect(actualCm.Annotations).To(Equal(map[string]string{
			"annotation1": "value1",
			"annotation2": "cluster_value2",
			"annotation3": "value3",
		}))
	})

	It("should recreate object when ForceRecreate is set", func() {
		oldCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "force-recreate",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "old",
			},
		}
		Expect(cli.Sync(env.Context, oldCm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "force-recreate", Namespace: testNamespace}, actualCm)).To(Succeed())
		uid := actualCm.UID

		newCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "force-recreate",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "new",
			},
		}
		Expect(cli.Sync(env.Context, newCm, &clientwrapper.SyncOptions{DiffOpts: cmDiffs, ForceRecreate: true})).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "force-recreate", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"key": "new"}))
		Expect(actualCm.UID).NotTo(Equal(uid))
	})
})
