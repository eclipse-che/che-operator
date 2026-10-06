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

package provision

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
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
		cheContext *chetypes.CheContext
	)

	BeforeAll(func() {
		cheContext = env.NewCheCtxBuilder().WithEmptyCheCluster().Build()
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(cheContext.CheCluster.Namespace))).To(Succeed())
	})

	It("should set finalizer", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Namespace:  "eclipse-che",
				Finalizers: []string{},
			},
		}

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Namespace:  "eclipse-che",
				Finalizers: []string{"test/test"},
			},
		}

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Finalizers).To(Equal([]string{"test/test"}))

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
		}

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Finalizers).To(BeEmpty())
	})

	It("should set owner reference", func(ctx SpecContext) {
		testCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
		}

		emptyCheClusterContext := env.NewCheCtxBuilder().Build()
		Expect(syncObject(emptyCheClusterContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.OwnerReferences).To(BeEmpty())

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.OwnerReferences).To(HaveLen(1))
		Expect(actualCm.OwnerReferences[0].UID).To(Equal(cheContext.CheCluster.UID))
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

		Expect(syncObject(cheContext, testCm, cmDiffs, false, true)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"a": "b"}))

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data:      map[string]string{"a": "c"},
			Immutable: new(true),
		}

		Expect(syncObject(cheContext, testCm, cmDiffs, false, true)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
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

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		testCm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "eclipse-che",
			},
			Data:      map[string]string{"a": "c"},
			Immutable: new(true),
		}

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Not(Succeed()))
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

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

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

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
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

		Expect(syncObject(cheContext, testCm, cmDiffs, false, false)).To(Succeed())

		actualCm = &corev1.ConfigMap{}
		Expect(env.Client.Get(cheContext.Context, types.NamespacedName{Name: "test", Namespace: "eclipse-che"}, actualCm)).To(Succeed())
		Expect(actualCm.Labels["label1"]).To(Equal("value2"))
		Expect(actualCm.Labels["label2"]).To(Equal("value3"))
		Expect(actualCm.Labels["label3"]).To(Equal("value2"))
		Expect(actualCm.Annotations["annotation1"]).To(Equal("value2"))
		Expect(actualCm.Annotations["annotation2"]).To(Equal("value3"))
		Expect(actualCm.Annotations["annotation3"]).To(Equal("value2"))
	})
})
