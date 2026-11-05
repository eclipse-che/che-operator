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
	chev2 "github.com/eclipse-che/che-operator/api/v2"
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

var cheClusterStatusDiffs = cmp.Options{
	cmpopts.IgnoreFields(chev2.CheCluster{}, "TypeMeta", "Spec", "ObjectMeta"),
}

var _ = Describe("SyncStatus", Ordered, func() {
	var clientWrapper *clientwrapper.K8sClientWrapper

	BeforeAll(func() {
		clientWrapper = clientwrapper.New(env.Client, env.Scheme)
		Expect(env.Client.Create(env.Context, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che"}})).To(Succeed())
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &chev2.CheCluster{}, client.InNamespace("eclipse-che"))).To(Succeed())
	})

	It("should update status when it is not in sync", func() {
		cheCluster := &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
		}
		Expect(env.Client.Create(env.Context, cheCluster)).To(Succeed())

		cheCluster.Status = chev2.CheClusterStatus{CheURL: "https://new-host"}
		err := clientWrapper.SyncStatus(env.Context, cheCluster, &clientwrapper.SyncOptions{DiffOpts: cheClusterStatusDiffs})
		Expect(err).To(Succeed())

		actualCheCluster := &chev2.CheCluster{}
		key := types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"}
		Expect(env.Client.Get(env.Context, key, actualCheCluster)).To(Succeed())
		Expect(actualCheCluster.Status.CheURL).To(Equal("https://new-host"))
	})

	It("should not update status when it is in sync", func() {
		cheCluster := &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Status: chev2.CheClusterStatus{
				CheURL: "https://old-host",
			},
		}
		Expect(env.Client.Create(env.Context, cheCluster)).To(Succeed())

		err := clientWrapper.SyncStatus(env.Context, cheCluster, &clientwrapper.SyncOptions{DiffOpts: cheClusterStatusDiffs})
		Expect(err).To(Succeed())
	})

	It("should not update spec when status is not in sync", func() {
		cheCluster := &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
		}
		Expect(env.Client.Create(env.Context, cheCluster)).To(Succeed())

		cheCluster.Status = chev2.CheClusterStatus{CheURL: "https://new-host"}
		cheCluster.Spec = chev2.CheClusterSpec{ContainerRegistry: chev2.CheClusterContainerRegistry{Hostname: "new-host"}}
		err := clientWrapper.SyncStatus(env.Context, cheCluster, &clientwrapper.SyncOptions{DiffOpts: cheClusterStatusDiffs})
		Expect(err).To(Succeed())

		actualCheCluster := &chev2.CheCluster{}
		key := types.NamespacedName{Name: "eclipse-che", Namespace: "eclipse-che"}
		Expect(env.Client.Get(env.Context, key, actualCheCluster)).To(Succeed())
		Expect(actualCheCluster.Status.CheURL).To(Equal("https://new-host"))
		Expect(actualCheCluster.Spec.ContainerRegistry.Hostname).To(BeEmpty())
	})
})
