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

package labelbackfill

import (
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestLabelBackfill(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LabelBackfill Reconciler Suite")
}

var _ = Describe("LabelBackfillReconciler ConsoleLink", func() {
	var (
		cheCtx     *chetypes.CheContext
		reconciler *LabelBackfillReconciler
	)

	BeforeEach(func() {
		test.Initialize(infrastructure.OpenShiftV4)
		reconciler = NewReconciler()
		cheCtx = test.NewCtxBuilder().Build()
	})

	Context("Reconcile", func() {
		Context("when the ConsoleLink exists without the part-of label", func() {
			var consoleLink *consolev1.ConsoleLink

			BeforeEach(func() {
				consoleLink = &consolev1.ConsoleLink{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConsoleLink",
						APIVersion: consolev1.GroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: defaults.GetConsoleLinkName(),
					},
				}

				cheCtx = test.NewCtxBuilder().WithObjects(consoleLink).Build()
			})

			It("should add the part-of label to the ConsoleLink", func() {
				_, done, err := reconciler.Reconcile(cheCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeTrue())

				updated := &consolev1.ConsoleLink{}
				key := types.NamespacedName{Name: defaults.GetConsoleLinkName()}
				Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, key, updated)).To(Succeed())
				Expect(updated.GetLabels()[constants.KubernetesPartOfLabelKey]).To(Equal(constants.CheEclipseOrg))
			})
		})
	})
})
