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

package envtest

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func ExpectReconciled(
	cheCtx *chetypes.CheContext,
	reconcileFunc func(cheCtx *chetypes.CheContext) (result reconcile.Result, done bool, err error)) {

	result, done, err := reconcileFunc(cheCtx)
	Expect(err).ToNot(HaveOccurred())
	if done {
		Expect(result).To(Equal(reconcile.Result{}))
		return
	}

	result, done, err = reconcileFunc(cheCtx)
	Expect(err).ToNot(HaveOccurred())
	Expect(done).To(BeTrue())
	Expect(result).To(Equal(reconcile.Result{}))
}
