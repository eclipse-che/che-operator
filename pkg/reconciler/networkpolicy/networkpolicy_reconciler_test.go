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

package networkpolicy

import (
	"fmt"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("NetworkPoliciesReconciler", func() {
	var (
		cheCtx     *chetypes.CheContext
		reconciler *NetworkPoliciesReconciler
	)

	BeforeEach(func() {
		cheCtx = test.NewCheContextBuilder().WithEmptyCheCluster().Build()
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &networkingv1.NetworkPolicy{}, client.InNamespace(cheCtx.CheCluster.Namespace))).To(Succeed())
		Expect(env.Client.DeleteAllOf(env.Context, &chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "eclipse-che"}}, client.InNamespace(cheCtx.CheCluster.Namespace))).To(Succeed())
	})

	Context("Finalize", func() {
		BeforeEach(func() {
			test.InitializeDefaults(infrastructure.Kubernetes)
		})

		It("should report the component as finalized", func() {
			Expect(reconciler.Finalize(cheCtx)).To(Succeed())
		})
	})

	Context("when NetworkPolicy is disabled", func() {
		BeforeEach(func() {
			test.InitializeDefaults(infrastructure.Kubernetes)
			cheCtx.CheCluster.Spec.Networking.NetworkPolicy = &chev2.NetworkPolicy{Enabled: new(false)}
		})

		It("should delete NetworkPolicies created while enabled", func() {
			By("provisioning NetworkPolicies as if enabled")
			Expect(reconciler.syncAll(cheCtx)).To(Not(HaveOccurred()))

			By("confirming the NetworkPolicies exist")
			npList := &networkingv1.NetworkPolicyList{}
			Expect(cheCtx.ClusterAPI.Client.List(cheCtx.Context, npList)).To(Succeed())
			Expect(npList.Items).To(Not(BeEmpty()))

			By("reconciling")
			Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

			By("confirming the NetworkPolicies are gone")
			npList = &networkingv1.NetworkPolicyList{}
			Expect(cheCtx.ClusterAPI.Client.List(cheCtx.Context, npList)).To(Succeed())
			Expect(npList.Items).To(BeEmpty())
		})
	})

	Context("when NetworkPolicy is enabled", func() {
		BeforeEach(func() {
			cheCtx.CheCluster.Spec.Networking.NetworkPolicy = &chev2.NetworkPolicy{Enabled: new(true)}
		})

		Context("on OpenShift", func() {
			var expectedPolicyNames []string

			BeforeEach(func() {
				test.InitializeDefaults(infrastructure.OpenShiftV4)

				expectedPolicyNames = []string{
					"allow-from-same-namespace",
					"allow-from-openshift-ingress",
					"allow-from-openshift-host-network",
					"allow-from-openshift-monitoring",
					"allow-from-workspaces",
					fmt.Sprintf("allow-from-%s-operator", defaults.GetCheFlavor()),
					"allow-all-egress",
				}
			})

			It("should create the scoped OpenShift ingress policies and allow-all egress", func() {
				Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

				npList := &networkingv1.NetworkPolicyList{}
				Expect(cheCtx.ClusterAPI.Client.List(cheCtx.Context, npList)).To(Succeed())
				Expect(npList.Items).To(HaveLen(len(expectedPolicyNames)))

				npNames := utils.Map(npList.Items, func(np networkingv1.NetworkPolicy) string { return np.Name })
				Expect(npNames).To(ConsistOf(expectedPolicyNames))
			})
		})

		Context("on Kubernetes", func() {
			var expectedPolicyNames []string

			BeforeEach(func() {
				test.InitializeDefaults(infrastructure.Kubernetes)

				expectedPolicyNames = []string{
					"allow-all-egress",
					"allow-all-ingress",
				}
			})

			It("should create permissive allow-all ingress and egress policies", func() {
				Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

				npList := &networkingv1.NetworkPolicyList{}
				Expect(cheCtx.ClusterAPI.Client.List(cheCtx.Context, npList)).To(Succeed())
				Expect(npList.Items).To(HaveLen(len(expectedPolicyNames)))

				npNames := utils.Map(npList.Items, func(np networkingv1.NetworkPolicy) string { return np.Name })
				Expect(npNames).To(ConsistOf(expectedPolicyNames))
			})

			It("should revert out-of-band edits to a NetworkPolicy", func() {
				Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

				npKey := types.NamespacedName{Name: "allow-all-ingress", Namespace: cheCtx.CheCluster.Namespace}

				By("editing allow-all-ingress out of band")
				np := &networkingv1.NetworkPolicy{}
				Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, npKey, np)).To(Succeed())
				Expect(np.Spec.Ingress).To(Not(BeNil()))

				np.Spec.Ingress = nil
				Expect(cheCtx.ClusterAPI.Client.Update(cheCtx.Context, np)).To(Succeed())

				By("reading it back before reconciling")
				np = &networkingv1.NetworkPolicy{}
				Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, npKey, np)).To(Succeed())
				Expect(np.Spec.Ingress).To(BeNil())

				By("reconciling a second time")
				Expect(test.ReconcileTwice(cheCtx, reconciler.Reconcile)).To(Succeed())

				By("reading it back after reconciling")
				np = &networkingv1.NetworkPolicy{}
				Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, npKey, np)).To(Succeed())
				Expect(np.Spec.Ingress).To(Not(BeNil()))
			})
		})
	})
})
