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
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TlsSecretReconciler struct {
	reconciler.Reconcilable
}

func NewTlsSecretReconciler() *TlsSecretReconciler {
	return &TlsSecretReconciler{}
}

func (t *TlsSecretReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if infrastructure.IsOpenShift() {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if cheCtx.IsSelfSignedCertificate {
			if err := CreateTLSSecret(cheCtx, constants.DefaultSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	} else {
		// Handle Che TLS certificates on Kubernetes infrastructure
		if cheCtx.CheCluster.Spec.Networking.TlsSecretName != "" {
			// Self-signed certificate should be created to secure Che ingresses
			result, err := K8sHandleCheTLSSecrets(cheCtx)
			if result.RequeueAfter > 0 {
				return result, false, err
			}
		} else if cheCtx.IsSelfSignedCertificate {
			// Use default self-signed ingress certificate
			if err := CreateTLSSecret(cheCtx, constants.DefaultSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	}

	return reconcile.Result{}, true, nil
}

func (t *TlsSecretReconciler) Finalize(cheCtx *chetypes.CheContext) bool {
	return true
}
