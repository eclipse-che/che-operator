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

package consolelink

import (
	"fmt"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("ConsoleLinkReconciler", func() {
	var (
		cheCtx     *chetypes.CheContext
		reconciler *ConsoleLinkReconciler
	)

	BeforeEach(func() {
		test.Initialize(infrastructure.OpenShiftV4)
		reconciler = NewReconciler()
	})

	Context("Reconcile", func() {
		BeforeEach(func() {
			cheCtx = test.NewCtxBuilder().Build()
		})

		It("should create the ConsoleLink", func() {
			test.Reconcile(cheCtx, reconciler.Reconcile)

			consoleLink := &consolev1.ConsoleLink{}
			key := types.NamespacedName{Name: defaults.GetConsoleLinkName()}
			Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, key, consoleLink)).To(Succeed())
			Expect(consoleLink.Spec.Href).To(Equal("https://che-host"))
			Expect(consoleLink.Spec.ApplicationMenu.ImageURL).To(Equal(fmt.Sprintf("https://che-host%s", defaults.GetConsoleLinkImage())))
		})
	})

	Context("when the Che URL changes", func() {
		BeforeEach(func() {
			cheCluster := &chev2.CheCluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "eclipse-che",
					Name:      "eclipse-che",
				},
				Status: chev2.CheClusterStatus{
					CheURL: "https://test-host",
				},
			}

			consoleLink := &consolev1.ConsoleLink{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConsoleLink",
					APIVersion: consolev1.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: defaults.GetConsoleLinkName(),
				},
				Spec: consolev1.ConsoleLinkSpec{
					Link: consolev1.Link{
						Href: "https://che-host",
						Text: defaults.GetConsoleLinkDisplayName()},
					Location: consolev1.ApplicationMenu,
					ApplicationMenu: &consolev1.ApplicationMenuSpec{
						Section:  defaults.GetConsoleLinkSection(),
						ImageURL: fmt.Sprintf("https://%s%s", "che-host", defaults.GetConsoleLinkImage()),
					},
				},
			}

			cheCtx = test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(consoleLink).Build()
		})

		It("should update the ConsoleLink href and image URL", func() {
			test.Reconcile(cheCtx, reconciler.Reconcile)

			consoleLink := &consolev1.ConsoleLink{}
			key := types.NamespacedName{Name: defaults.GetConsoleLinkName()}
			Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, key, consoleLink)).To(Succeed())
			Expect(consoleLink.Spec.Href).To(Equal("https://test-host"))
			Expect(consoleLink.Spec.ApplicationMenu.ImageURL).To(Equal(fmt.Sprintf("https://test-host%s", defaults.GetConsoleLinkImage())))
		})
	})
})
