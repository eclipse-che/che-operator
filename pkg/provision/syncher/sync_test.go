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

package syncher

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cmDiffs = cmp.Options{cmpopts.IgnoreFields(corev1.ConfigMap{}, "TypeMeta")}
)

var _ = Describe("syncObject", Ordered, func() {
	var (
		cheCtx *chetypes.CheContext
	)

	BeforeAll(func() {
		cheCtx = env.NewCheCtxBuilder().WithEmptyCheCluster().Build()
		test.Initialize(infrastructure.Kubernetes)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(cheCtx.CheCluster.Namespace))).To(Succeed())
	})

	It("should set owner reference", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
		}

		emptyCheClusterContext := env.NewCheCtxBuilder().Build()
		Expect(SyncObject(emptyCheClusterContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.OwnerReferences).To(BeEmpty())

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.OwnerReferences).To(HaveLen(1))
		Expect(actualCm.OwnerReferences[0].UID).To(Equal(cheCtx.CheCluster.UID))
	})

	It("should not set owner reference when syncing the CheCluster itself", func(ctx SpecContext) {
		cheClusterDiffs := cmp.Options{cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "Status")}
		Expect(SyncObject(cheCtx, cheCtx.CheCluster, cheClusterDiffs, false, false)).To(Succeed())

		actualCheCluster := &chev2.CheCluster{}
		key := types.NamespacedName{Name: cheCtx.CheCluster.Name, Namespace: cheCtx.CheCluster.Namespace}
		Expect(env.Client.Get(cheCtx.Context, key, actualCheCluster)).To(Succeed())
		Expect(actualCheCluster.OwnerReferences).To(BeEmpty())
	})

	It("should sync finalizers", func(ctx SpecContext) {
		cheClusterDiffs := cmp.Options{cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "Status")}
		key := types.NamespacedName{Name: cheCtx.CheCluster.Name, Namespace: cheCtx.CheCluster.Namespace}

		cheCtx.CheCluster.Finalizers = []string{"a.finalizers.che.eclipse.org", "b.finalizers.che.eclipse.org"}
		Expect(SyncObject(cheCtx, cheCtx.CheCluster, cheClusterDiffs, false, false)).To(Succeed())

		actualCheCluster := &chev2.CheCluster{}
		Expect(env.Client.Get(cheCtx.Context, key, actualCheCluster)).To(Succeed())
		Expect(actualCheCluster.Finalizers).To(ConsistOf("a.finalizers.che.eclipse.org", "b.finalizers.che.eclipse.org"))
		resourceVersion := actualCheCluster.ResourceVersion

		// the same finalizers in a different order should not produce an update
		cheCtx.CheCluster.Finalizers = []string{"b.finalizers.che.eclipse.org", "a.finalizers.che.eclipse.org"}
		Expect(SyncObject(cheCtx, cheCtx.CheCluster, cheClusterDiffs, false, false)).To(Succeed())

		actualCheCluster = &chev2.CheCluster{}
		Expect(env.Client.Get(cheCtx.Context, key, actualCheCluster)).To(Succeed())
		Expect(actualCheCluster.Finalizers).To(ConsistOf("a.finalizers.che.eclipse.org", "b.finalizers.che.eclipse.org"))
		Expect(actualCheCluster.ResourceVersion).To(Equal(resourceVersion))

		cheCtx.CheCluster.Finalizers = nil
		Expect(SyncObject(cheCtx, cheCtx.CheCluster, cheClusterDiffs, false, false)).To(Succeed())

		actualCheCluster = &chev2.CheCluster{}
		Expect(env.Client.Get(cheCtx.Context, key, actualCheCluster)).To(Succeed())
		Expect(actualCheCluster.Finalizers).To(BeEmpty())
	})

	It("should not diff or strip finalizers set by other controllers", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data: map[string]string{"a": "b"},
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		// a foreign finalizer is added to the existing object by another controller;
		// finalizers on built-in types are validated as label keys, so the name needs the 'domain/name' form
		actualCm := &corev1.ConfigMap{}
		key := types.NamespacedName{Name: "test", Namespace: "eclipse-che"}
		Expect(env.Client.Get(cheCtx.Context, key, actualCm)).To(Succeed())
		actualCm.Finalizers = []string{"example.org/foreign-finalizer"}
		Expect(env.Client.Update(cheCtx.Context, actualCm)).To(Succeed())

		DeferCleanup(
			func(ctx SpecContext) {
				cm := &corev1.ConfigMap{}
				Expect(env.Client.Get(ctx, key, cm)).To(Succeed())
				cm.Finalizers = nil
				Expect(env.Client.Update(ctx, cm)).To(Succeed())
			},
		)

		// syncing an object without finalizers neither produces an update nor strips the finalizer
		resourceVersion := actualCm.ResourceVersion
		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, key, actualCm)).To(Succeed())
		Expect(actualCm.Finalizers).To(ConsistOf("example.org/foreign-finalizer"))
		Expect(actualCm.ResourceVersion).To(Equal(resourceVersion))

		// an update triggered by a spec change preserves the finalizer as well;
		// the desired object is built fresh on each reconcile and carries no resource
		// version, so the sync adopts the latest one from the cluster
		testCm.ResourceVersion = ""
		testCm.Data = map[string]string{"a": "c"}
		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		Expect(env.Client.Get(cheCtx.Context, key, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"a": "c"}))
		Expect(actualCm.Finalizers).To(ConsistOf("example.org/foreign-finalizer"))
		Expect(actualCm.ResourceVersion).ToNot(Equal(resourceVersion))
	})

	It("should sync immutable object", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data:      map[string]string{"a": "b"},
			Immutable: new(true),
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, true)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"a": "b"}))

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data:      map[string]string{"a": "c"},
			Immutable: new(true),
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, true)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"a": "c"}))
	})

	It("should not sync immutable object", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data:      map[string]string{"a": "b"},
			Immutable: new(true),
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data:      map[string]string{"a": "c"},
			Immutable: new(true),
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Not(Succeed()))
	})

	It("should respect metadata", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"label1": "value1",
					"label2": "value1",
				},
				Annotations: map[string]string{
					"annotation1": "value1",
					"annotation2": "value1",
				},
			},
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"label1": "value2",
					"label3": "value2",
				},
				Annotations: map[string]string{
					"annotation1": "value2",
					"annotation3": "value2",
				},
			},
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Labels["label1"]).To(Equal("value2"))
		Expect(actualCm.Labels["label2"]).To(Equal("value1"))
		Expect(actualCm.Labels["label3"]).To(Equal("value2"))
		Expect(actualCm.Annotations["annotation1"]).To(Equal("value2"))
		Expect(actualCm.Annotations["annotation2"]).To(Equal("value1"))
		Expect(actualCm.Annotations["annotation3"]).To(Equal("value2"))

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"label2": "value3",
				},
				Annotations: map[string]string{
					"annotation2": "value3",
				},
			},
		}

		Expect(SyncObject(cheCtx, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheCtx.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Labels["label1"]).To(Equal("value2"))
		Expect(actualCm.Labels["label2"]).To(Equal("value3"))
		Expect(actualCm.Labels["label3"]).To(Equal("value2"))
		Expect(actualCm.Annotations["annotation1"]).To(Equal("value2"))
		Expect(actualCm.Annotations["annotation2"]).To(Equal("value3"))
		Expect(actualCm.Annotations["annotation3"]).To(Equal("value2"))
	})
})
