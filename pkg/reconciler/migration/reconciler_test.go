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

package migration

import (
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	consolev1 "github.com/openshift/api/console/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("LabelsMigrationReconciler", func() {
	var (
		cheCtx     *chetypes.CheContext
		reconciler *MigrationReconciler
	)

	BeforeEach(func() {
		test.Initialize(infrastructure.OpenShiftV4)
		reconciler = NewReconciler()
	})

	Context("Reconcile", func() {
		Context("when known objects exist without the part-of label", func() {
			var selfSignedCertificateSecret *corev1.Secret

			BeforeEach(func() {
				selfSignedCertificateSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.SelfSignedCertificateSecretName,
						Namespace: "eclipse-che",
					},
				}
				caBundleConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.DefaultCaBundleCertsCMName,
						Namespace: "eclipse-che",
					},
				}
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.DefaultCheTLSSecretName,
						Namespace: "eclipse-che",
					},
				}
				proxyCredentialsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.DefaultProxyCredentialsSecret,
						Namespace: "eclipse-che",
					},
				}
				gitTrustedCertsConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      constants.DefaultGitSelfSignedCertsConfigMapName,
						Namespace: "eclipse-che",
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
				}

				cheCtx = test.NewCtxBuilder().WithObjects(
					selfSignedCertificateSecret,
					caBundleConfigMap,
					tlsSecret,
					proxyCredentialsSecret,
					gitTrustedCertsConfigMap,
					consoleLink,
				).Build()
			})

			It("should add the part-of label to all known objects", func() {
				_, done, err := reconciler.Reconcile(cheCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeTrue())

				validatePartOfLabel(
					cheCtx,
					types.NamespacedName{
						Namespace: "eclipse-che",
						Name:      constants.SelfSignedCertificateSecretName,
					},
					&corev1.Secret{},
				)
				validatePartOfLabel(
					cheCtx,
					types.NamespacedName{
						Namespace: "eclipse-che",
						Name:      constants.DefaultCaBundleCertsCMName,
					},
					&corev1.ConfigMap{},
				)
				validatePartOfLabel(
					cheCtx,
					types.NamespacedName{
						Namespace: "eclipse-che",
						Name:      constants.DefaultCheTLSSecretName,
					},
					&corev1.Secret{},
				)
				validatePartOfLabel(
					cheCtx,
					types.NamespacedName{
						Namespace: "eclipse-che",
						Name:      constants.DefaultProxyCredentialsSecret,
					},
					&corev1.Secret{},
				)
				validatePartOfLabel(
					cheCtx,
					types.NamespacedName{
						Namespace: "eclipse-che",
						Name:      constants.DefaultGitSelfSignedCertsConfigMapName,
					},
					&corev1.ConfigMap{},
				)
				validatePartOfLabel(
					cheCtx,
					types.NamespacedName{
						Name: defaults.GetConsoleLinkName(),
					},
					&consolev1.ConsoleLink{},
				)
			})

			It("should preserve existing labels when adding the part-of label", func() {
				selfSignedCertificateSecret.Labels = map[string]string{"custom": "value"}
				Expect(cheCtx.ClusterAPI.Client.Update(cheCtx.Context, selfSignedCertificateSecret)).To(Succeed())

				_, done, err := reconciler.Reconcile(cheCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeTrue())

				secret := &corev1.Secret{}
				key := types.NamespacedName{Namespace: "eclipse-che", Name: constants.SelfSignedCertificateSecretName}
				Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, key, secret)).To(Succeed())
				Expect(secret.Labels["custom"]).To(Equal("value"))
				Expect(secret.Labels[constants.KubernetesPartOfLabelKey]).To(Equal(constants.CheEclipseOrg))
			})
		})
	})
})

func validatePartOfLabel(cheCtx *chetypes.CheContext, key types.NamespacedName, obj client.Object) {
	Expect(cheCtx.ClusterAPI.Client.Get(cheCtx.Context, key, obj)).To(Succeed())
	Expect(obj.GetLabels()[constants.KubernetesPartOfLabelKey]).To(Equal(constants.CheEclipseOrg))
}
