//
// Copyright (c) 2012-2021 Red Hat, Inc.
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
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TlsSecretReconciler struct {
	deploy.Reconcilable
}

func NewTlsSecretReconciler() *TlsSecretReconciler {
	return &TlsSecretReconciler{}
}

func (t *TlsSecretReconciler) Reconcile(ctx *deploy.DeployContext) (reconcile.Result, bool, error) {
	if util.IsOpenShift {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if ctx.IsSelfSignedCertificate {
			if err := CreateTLSSecretFromEndpoint(ctx, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	} else {
		// Handle Che TLS certificates on Kubernetes infrastructure
		if ctx.CheCluster.Spec.K8s.TlsSecretName != "" {
			// Self-signed certificate should be created to secure Che ingresses
			result, err := K8sHandleCheTLSSecrets(ctx)
			if result.Requeue || result.RequeueAfter > 0 {
				return result, false, err
			}
		} else if ctx.IsSelfSignedCertificate {
			// Use default self-signed ingress certificate
			if err := CreateTLSSecretFromEndpoint(ctx, "", deploy.CheTLSSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	}

	return reconcile.Result{}, true, nil
}

func (t *TlsSecretReconciler) Finalize(ctx *deploy.DeployContext) bool {
	return true
}
