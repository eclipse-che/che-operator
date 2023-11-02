//
// Copyright (c) 2019-2023 Red Hat, Inc.
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
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TlsSecretReconciler struct {
	deploy.Reconcilable
}

func NewTlsSecretReconciler() *TlsSecretReconciler {
	return &TlsSecretReconciler{}
}

func (t *TlsSecretReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if infrastructure.IsOpenShift() {
		// create a secret with router tls cert when on OpenShift infra and router is configured with a self signed certificate
		if ctx.IsSelfSignedCertificate {
			if err := CreateTLSSecret(ctx, constants.DefaultSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	} else {
		// Handle Che TLS certificates on Kubernetes infrastructure
		if ctx.CheCluster.Spec.Networking.TlsSecretName != "" {
			// Self-signed certificate should be created to secure Che ingresses
			result, err := K8sHandleCheTLSSecrets(ctx)
			if result.Requeue || result.RequeueAfter > 0 {
				return result, false, err
			}
		} else if ctx.IsSelfSignedCertificate {
			// Use default self-signed ingress certificate
			if err := CreateTLSSecret(ctx, constants.DefaultSelfSignedCertificateSecretName); err != nil {
				return reconcile.Result{}, false, err
			}
		}
	}

	return reconcile.Result{}, true, nil
}

func (t *TlsSecretReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}
