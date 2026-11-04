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
	"time"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SelfSignedCertificateReconciler", func() {
	var reconciler *SelfSignedCertificateReconciler

	BeforeEach(func() {
		test.Initialize(infrastructure.Kubernetes)
		reconciler = NewReconciler()
	})

	It("is done when custom TLS secret exists", func() {
		tlsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-tls-secret",
				Namespace: "eclipse-che",
			},
			Data: map[string][]byte{"tls.crt": []byte("crt")},
		}
		cheCtx := test.NewCtxBuilder().
			WithCheCluster(
				&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Networking: chev2.CheClusterSpecNetworking{
							TlsSecretName: "custom-tls-secret",
						},
					},
				}).
			WithObjects(tlsSecret).
			Build()

		result, done, err := reconciler.Reconcile(cheCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(done).To(BeTrue())
		Expect(result).To(Equal(reconcile.Result{}))

		job := &batchv1.Job{}
		err = cheCtx.ClusterAPI.Client.Get(cheCtx.Context, types.NamespacedName{Name: cheTLSJobName, Namespace: cheCtx.CheCluster.Namespace}, job)
		Expect(err).To(HaveOccurred())
	})

	It("creates TLS creation job when custom TLS secret is missing", func() {
		cheCtx := test.NewCtxBuilder().
			WithCheCluster(
				&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Networking: chev2.CheClusterSpecNetworking{
							TlsSecretName: "custom-tls-secret",
						},
					},
				}).
			Build()

		result, done, err := reconciler.Reconcile(cheCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(done).To(BeFalse())
		Expect(result.RequeueAfter).To(Equal(5 * time.Second))

		job := &batchv1.Job{}
		err = cheCtx.ClusterAPI.Client.Get(cheCtx.Context, types.NamespacedName{Name: cheTLSJobName, Namespace: cheCtx.CheCluster.Namespace}, job)
		Expect(err).ToNot(HaveOccurred())
		Expect(job.Spec.Template.Spec.ServiceAccountName).To(Equal(cheTLSJobServiceAccountName))
	})

	It("is done when self-signed certificate secret already exists", func() {
		selfSignedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.SelfSignedCertificateSecretName,
				Namespace: "eclipse-che",
			},
			Data: map[string][]byte{"ca.crt": []byte("existing-ca")},
		}
		cheCtx := test.NewCtxBuilder().
			WithCheCluster(
				&chev2.CheCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "eclipse-che",
						Namespace: "eclipse-che",
					},
					Spec: chev2.CheClusterSpec{
						Networking: chev2.CheClusterSpecNetworking{
							TlsSecretName: "",
						},
					},
				}).
			WithObjects(selfSignedSecret).
			Build()

		result, done, err := reconciler.Reconcile(cheCtx)
		Expect(err).ToNot(HaveOccurred())
		Expect(done).To(BeTrue())
		Expect(result).To(Equal(reconcile.Result{}))

		secret := &corev1.Secret{}
		exists, err := resources.Secret.Get(cheCtx, constants.SelfSignedCertificateSecretName, cheCtx.CheCluster.Namespace, secret)
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
		Expect(secret.Data["ca.crt"]).To(Equal([]byte("existing-ca")))
	})
})
