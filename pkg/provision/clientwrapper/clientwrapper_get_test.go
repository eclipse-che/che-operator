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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GetIgnoreNotFound", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should return existing object with GVK set on it", func() {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		Expect(env.Client.Create(env.Context, cm)).To(Succeed())

		actualCm := &corev1.ConfigMap{}
		exists, err := cli.GetIgnoreNotFound(env.Context, client.ObjectKeyFromObject(cm), actualCm)

		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())
		Expect(actualCm.Data).To(Equal(map[string]string{"key": "value"}))
		Expect(actualCm.Kind).To(Equal("ConfigMap"))
		Expect(actualCm.APIVersion).To(Equal("v1"))
	})

	It("should not fail when object does not exist", func() {
		exists, err := cli.GetIgnoreNotFound(env.Context, types.NamespacedName{Name: "missing", Namespace: testNamespace}, &corev1.ConfigMap{})

		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeFalse())
	})
})
