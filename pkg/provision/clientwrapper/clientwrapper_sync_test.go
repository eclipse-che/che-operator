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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	It("should not merge labels and annotations into an object read from the cluster", func() {
		existing := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "no-merge-fetched",
				Namespace: testNamespace,
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value2",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation2": "value2",
				},
			},
		}
		Expect(cli.Sync(env.Context, existing, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		// read the object, so that it carries a resourceVersion, and remove keys on purpose
		fetched := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "no-merge-fetched", Namespace: testNamespace}, fetched)).To(Succeed())
		delete(fetched.Labels, "label2")
		delete(fetched.Annotations, "annotation2")
		fetched.Annotations["annotation3"] = "value3"

		Expect(cli.Sync(env.Context, fetched, &clientwrapper.SyncOptions{DiffOpts: cmDiffs, MergeLabels: true, MergeAnnotations: true})).To(Succeed())

		// the removed keys must not be restored from the existing object
		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "no-merge-fetched", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Labels).To(Equal(map[string]string{"label1": "value1"}))
		Expect(actualCm.Annotations).To(Equal(map[string]string{"annotation1": "value1", "annotation3": "value3"}))
	})

	It("should fail with a conflict when an object read from the cluster has been changed since", func() {
		existing := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "conflict-fetched",
				Namespace: testNamespace,
				Annotations: map[string]string{
					"annotation1": "value1",
				},
			},
		}
		Expect(cli.Sync(env.Context, existing, &clientwrapper.SyncOptions{DiffOpts: cmDiffs})).To(Succeed())

		stale := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "conflict-fetched", Namespace: testNamespace}, stale)).To(Succeed())

		// someone else adds an annotation after the object has been read
		other := stale.DeepCopy()
		other.Annotations["annotation2"] = "value2"
		Expect(env.Client.Update(env.Context, other)).To(Succeed())

		// the stale object is not merged, so writing it would drop the other annotation;
		// the API server must reject it instead
		delete(stale.Annotations, "annotation1")
		err := cli.Sync(env.Context, stale, &clientwrapper.SyncOptions{DiffOpts: cmDiffs, MergeAnnotations: true})
		Expect(apierrors.IsConflict(err)).To(BeTrue(), "expected a conflict error, got: %v", err)

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "conflict-fetched", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Annotations).To(Equal(map[string]string{"annotation1": "value1", "annotation2": "value2"}))
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
