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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Create", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should create object and set GVK on it", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}

		Expect(cli.Create(env.Context, cm)).To(Succeed())
		Expect(cm.Kind).To(Equal("ConfigMap"))
		Expect(cm.APIVersion).To(Equal("v1"))

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "test", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"foo": "bar"}))
	})

	It("should fail when object already exists", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}
		Expect(cli.Create(env.Context, cm)).To(Succeed())

		duplicateCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}
		err := cli.Create(env.Context, duplicateCm)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
	})
})

var _ = Describe("CreateIfNotExists", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should create object when it does not exist", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}
		Expect(cli.CreateIfNotExists(env.Context, cm)).To(Succeed())
		Expect(cm.Kind).To(Equal("ConfigMap"))
		Expect(cm.APIVersion).To(Equal("v1"))

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "test", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"foo": "bar"}))
	})

	It("should keep existing object unchanged", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}
		Expect(cli.CreateIfNotExists(env.Context, cm)).To(Succeed())

		duplicateCm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		}
		Expect(cli.CreateIfNotExists(env.Context, duplicateCm)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		Expect(env.Client.Get(env.Context, types.NamespacedName{Name: "test", Namespace: testNamespace}, actualCm)).To(Succeed())
		Expect(actualCm.Data).To(Equal(map[string]string{"foo": "bar"}))
	})
})
