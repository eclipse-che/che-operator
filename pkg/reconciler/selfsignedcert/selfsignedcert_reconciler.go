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

package selfsignedcert

import (
	"fmt"
	"time"

	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	logger = ctrl.Log.WithName("selfsignedcert")
)

type SelfSignedCertificateReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *SelfSignedCertificateReconciler {
	return &SelfSignedCertificateReconciler{}
}

func (s *SelfSignedCertificateReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	result, done, err := s.syncAll(cheCtx)
	if err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to reconcile self-signed certificate secret: %w", err)
	}

	return result, done, nil
}

func (s *SelfSignedCertificateReconciler) Finalize(_ *chetypes.CheContext) error {
	return nil
}

// syncAll reconciles TLS certificates according to the environment:
//   - on OpenShift the serving certificate chain is always trusted, so only the
//     self-signed certificate secret may need to be created;
//   - on Kubernetes, when a custom TLS secret is configured, the Che TLS secret
//     and the self-signed CA certificate secret are ensured by a creation Job;
//   - otherwise the certificate chain is fetched from the Che endpoint and the
//     self-signed certificate secret is created if the chain is self-signed.
func (s *SelfSignedCertificateReconciler) syncAll(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if infrastructure.IsOpenShift() {
		return s.syncSelfSignedCertificateSecretIfNeeded(cheCtx)
	}

	if cheCtx.CheCluster.Spec.Networking.TlsSecretName != "" {
		return s.syncCheTLSSecrets(cheCtx)
	}
	return s.syncSelfSignedCertificateSecretIfNeeded(cheCtx)
}

// syncSelfSignedCertificateSecretIfNeeded fetches the certificate chain served by the
// Che endpoint and creates the self-signed certificate secret from it, unless the
// chain contains no self-signed certificate, in which case it is assumed to be
// trusted and no secret is required.
func (s *SelfSignedCertificateReconciler) syncSelfSignedCertificateSecretIfNeeded(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	needed, err := s.isSelfSignedCertificateFetchNeeded(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, err
	} else if !needed {
		return reconcile.Result{}, true, nil
	}

	// The certificate chain is not fetched yet, fetch it now.
	certificateChain, err := s.fetchTLSCertificateChain(cheCtx)
	if err != nil {
		if isTimeoutError(err) {
			logger.Info("Timed out fetching certificate chain from the Che endpoint, retrying")
			return reconcile.Result{RequeueAfter: time.Second}, false, nil
		}

		return reconcile.Result{}, false, err
	}

	if !containsSelfSignedCertificate(certificateChain) {
		// The served chain contains no self-signed certificate, so it is
		// assumed to be trusted; no self-signed certificate secret is required.
		return reconcile.Result{}, true, nil
	}

	crtBytes := encodeCertificateChainPEM(certificateChain)
	err = s.syncSelfSignedCertificateSecret(cheCtx, crtBytes)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (s *SelfSignedCertificateReconciler) syncSelfSignedCertificateSecret(cheCtx *chetypes.CheContext, crtBytes []byte) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SelfSignedCertificateSecretName,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    chelabels.GetStandardLabels(defaults.GetCheFlavor()),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"ca.crt": crtBytes},
	}
	return resources.Secret.Sync(cheCtx, secret)
}

// isSelfSignedCertificateFetchNeeded reports whether the certificate chain should be
// fetched from the Che endpoint to build the self-signed certificate secret:
//   - it is not needed if the secret `self-signed-certificate` already exists;
//   - it is not needed if the custom TLS secret is specified and found, since it is trusted;
//   - it is needed otherwise; the caller fetches the chain and creates the secret
//     only if the chain contains a self-signed certificate.
func (s *SelfSignedCertificateReconciler) isSelfSignedCertificateFetchNeeded(cheCtx *chetypes.CheContext) (bool, error) {
	exists, err := resources.Secret.Get(cheCtx, constants.SelfSignedCertificateSecretName, cheCtx.CheCluster.Namespace, &corev1.Secret{})
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	tlsSecretName := cheCtx.CheCluster.Spec.Networking.TlsSecretName
	if tlsSecretName != "" {
		exists, err = resources.Secret.Get(cheCtx, tlsSecretName, cheCtx.CheCluster.Namespace, &corev1.Secret{})
		if err != nil {
			return false, err
		}
		if exists {
			// Since the custom TLS secret is specified and found, it is trusted,
			// and no self-signed certificate secret is required.
			return false, nil
		}

		// The custom TLS secret is specified but missing; the caller fetches
		// the chain and creates the secret only if it is self-signed.
		return true, nil
	}

	return true, nil
}
