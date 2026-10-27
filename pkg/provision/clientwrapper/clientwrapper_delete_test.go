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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DeleteIgnoreNotFound", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should delete existing object", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
		}
		Expect(env.Client.Create(env.Context, cm)).To(Succeed())

		cm2delete := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
		}
		Expect(cli.DeleteIgnoreNotFound(env.Context, cm2delete)).To(Succeed())
		Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})).To(HaveOccurred())
	})

	It("should not fail when object does not exist", func() {
		cm2delete := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
		}
		Expect(cli.DeleteIgnoreNotFound(env.Context, cm2delete)).To(Succeed())
	})
})

var _ = Describe("DeleteByKeyIgnoreNotFound", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should delete existing object by key", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
		}
		Expect(env.Client.Create(env.Context, cm)).To(Succeed())

		cm2delete := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
		}
		Expect(cli.DeleteByKeyIgnoreNotFound(env.Context, client.ObjectKeyFromObject(cm2delete), &corev1.ConfigMap{})).To(Succeed())
		Expect(env.Client.Get(env.Context, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{})).To(HaveOccurred())
	})

	It("should not fail when object does not exist", func() {
		cm2delete := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "missing",
				Namespace: testNamespace,
			},
		}
		Expect(cli.DeleteByKeyIgnoreNotFound(env.Context, client.ObjectKeyFromObject(cm2delete), &corev1.ConfigMap{})).To(Succeed())
	})
})
