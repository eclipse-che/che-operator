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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("List", Ordered, func() {
	var cli *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		cli = clientwrapper.New(env.Client, env.Scheme)
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(testNamespace))).To(Succeed())
	})

	It("should return all objects of the given kind with GVK set on them", func() {
		for _, name := range []string{"test-1", "test-2", "test-3"} {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: testNamespace,
				},
			}
			Expect(env.Client.Create(env.Context, cm)).To(Succeed())
		}

		objs, err := cli.List(env.Context, &corev1.ConfigMapList{}, client.InNamespace(testNamespace))

		Expect(err).NotTo(HaveOccurred())
		Expect(objs).To(HaveLen(3))

		for _, obj := range objs {
			cm, ok := obj.(*corev1.ConfigMap)
			Expect(ok).To(BeTrue())
			Expect(cm.GroupVersionKind()).To(Equal(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}))
		}
	})
})
