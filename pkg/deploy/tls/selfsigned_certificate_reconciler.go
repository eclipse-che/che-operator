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

package tls

import (
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SelfSignedCertificateReconciler detects whether endpoints are secured
// by a self-signed certificate and sets cheCtx.IsSelfSignedCertificate accordingly.
//
// It must be registered after the Che host is reconciled (the Che route/ingress is exposed),
// since the certificate chain is retrieved from the Che endpoint.
type SelfSignedCertificateReconciler struct {
	reconciler.Reconcilable
}

func NewSelfSignedCertificateReconciler() *SelfSignedCertificateReconciler {
	return &SelfSignedCertificateReconciler{}
}

func (s *SelfSignedCertificateReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	isSelfSignedCertificate, err := IsSelfSignedCertificateUsed(cheCtx)
	if err != nil {
		return reconcile.Result{RequeueAfter: time.Second}, false, err
	}

	cheCtx.IsSelfSignedCertificate = isSelfSignedCertificate

	return reconcile.Result{}, true, nil
}

func (s *SelfSignedCertificateReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	return true
}
