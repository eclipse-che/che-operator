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

package reconciler

import (
	"errors"
	"testing"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcilerManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reconciler Manager Suite")
}

var _ = Describe("ReconcilerManager", func() {
	var (
		cheCtx *chetypes.CheContext
	)

	BeforeEach(func() {
		cheCtx = test.NewCtxBuilder().Build()
	})

	Context("ReconcileAll", func() {
		var (
			manager          *ReconcilerManager
			reconcilerCalled []bool
		)

		BeforeEach(func() {
			manager = NewReconcilerManager()
			reconcilerCalled = []bool{false, false, false}

			for i := range reconcilerCalled {
				index := i
				manager.AddReconciler(&mockReconciler{
					reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
						reconcilerCalled[index] = true
						return reconcile.Result{}, true, nil
					},
				})
			}
		})

		It("should reconcile all reconcilers when all succeed", func() {
			result, done, err := manager.ReconcileAll(cheCtx)

			Expect(err).ToNot(HaveOccurred())
			Expect(done).To(BeTrue())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(reconcilerCalled).To(Equal([]bool{true, true, true}))
		})
	})

	Context("ReconcileAll with a failing reconciler", func() {
		var (
			manager           *ReconcilerManager
			reconciler2Called bool
			reconciler3Called bool
		)

		BeforeEach(func() {
			manager = NewReconcilerManager()
			reconciler2Called = false
			reconciler3Called = false

			// First reconciler fails
			manager.AddReconciler(&mockReconciler{
				reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
					return reconcile.Result{}, false, errors.New("test")
				},
			})
			// These should not be called
			manager.AddReconciler(&mockReconciler{
				reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
					reconciler2Called = true
					return reconcile.Result{}, true, nil
				},
			})
			manager.AddReconciler(&mockReconciler{
				reconcileFunc: func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
					reconciler3Called = true
					return reconcile.Result{}, true, nil
				},
			})
		})

		It("should stop the chain and wrap the error", func() {
			result, done, err := manager.ReconcileAll(cheCtx)

			Expect(err).To(MatchError("reconciler.mockReconciler reconciliation failed: test"))
			Expect(done).To(BeFalse())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(reconciler2Called).To(BeFalse())
			Expect(reconciler3Called).To(BeFalse())
		})
	})

	Context("FinalizeAll", func() {
		var (
			manager          *ReconcilerManager
			reconcilerCalled []bool
		)

		BeforeEach(func() {
			manager = NewReconcilerManager()
			reconcilerCalled = []bool{false, false, false}

			for i := range reconcilerCalled {
				index := i
				manager.AddReconciler(&mockReconciler{
					finalizeFunc: func(cheCtx *chetypes.CheContext) error {
						reconcilerCalled[index] = true
						return nil
					},
				})
			}
		})

		It("should finalize all reconcilers when all succeed", func() {
			Expect(manager.FinalizeAll(cheCtx)).To(Succeed())
			Expect(reconcilerCalled).To(Equal([]bool{true, true, true}))
		})
	})

	Context("FinalizeAll with a failing reconciler", func() {
		var (
			manager           *ReconcilerManager
			reconciler1Called bool
			reconciler2Called bool
			reconciler3Called bool
		)

		BeforeEach(func() {
			manager = NewReconcilerManager()
			reconciler1Called = false
			reconciler2Called = false
			reconciler3Called = false

			manager.AddReconciler(&mockReconciler{
				finalizeFunc: func(cheCtx *chetypes.CheContext) error {
					reconciler1Called = true
					return nil
				},
			})
			// Second reconciler fails
			manager.AddReconciler(&mockReconciler{
				finalizeFunc: func(cheCtx *chetypes.CheContext) error {
					reconciler2Called = true
					return errors.New("test")
				},
			})
			// Third should still be called even though second failed
			manager.AddReconciler(&mockReconciler{
				finalizeFunc: func(cheCtx *chetypes.CheContext) error {
					reconciler3Called = true
					return nil
				},
			})
		})

		It("should collect the errors of failed finalizers", func() {
			err := manager.FinalizeAll(cheCtx)

			Expect(reconciler1Called).To(BeTrue())
			Expect(reconciler2Called).To(BeTrue())
			Expect(reconciler3Called).To(BeTrue())
			Expect(err).To(MatchError(ContainSubstring("reconciler.mockReconciler finalization failed: test")))
		})
	})
})

// mockReconciler is a mock implementation of Reconcilable for testing
type mockReconciler struct {
	reconcileFunc func(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error)
	finalizeFunc  func(cheCtx *chetypes.CheContext) error
}

func (m *mockReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if m.reconcileFunc != nil {
		return m.reconcileFunc(cheCtx)
	}
	return reconcile.Result{}, true, nil
}

func (m *mockReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	if m.finalizeFunc != nil {
		return m.finalizeFunc(cheCtx)
	}
	return nil
}
