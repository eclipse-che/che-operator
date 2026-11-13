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

//go:build integration

package labelbackfill

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/test/envtest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var env *envtest.Env

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	env = envtest.Start()
})

var _ = AfterSuite(func() {
	if env != nil {
		env.Stop()
	}
})

var _ = Describe("LabelBackfillReconciler", func() {
	var (
		cheCtx     *chetypes.CheContext
		reconciler *LabelBackfillReconciler
	)

	BeforeEach(func() {
		test.Initialize(infrastructure.Kubernetes)
		reconciler = NewReconciler()
	})

	AfterEach(func() {
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.Secret{}, client.InNamespace(cheCtx.CheCluster.Namespace))).To(Succeed())
		Expect(env.Client.DeleteAllOf(env.Context, &corev1.ConfigMap{}, client.InNamespace(cheCtx.CheCluster.Namespace))).To(Succeed())
		Expect(env.Client.DeleteAllOf(env.Context, &chev2.CheCluster{}, client.InNamespace(cheCtx.CheCluster.Namespace))).To(Succeed())
	})

	Context("Reconcile", func() {
		Context("when known objects exist without the part-of label", func() {
			BeforeEach(func() {
				cheCtx = env.NewCheCtxBuilder().
					WithCheCluster(&chev2.CheCluster{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "eclipse-che",
							Namespace: "eclipse-che",
						},
					}).
					WithObjects(
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      constants.SelfSignedCertificateSecretName,
								Namespace: "eclipse-che",
							},
						},
						&corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Name:      constants.DefaultCaBundleCertsCMName,
								Namespace: "eclipse-che",
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      constants.DefaultCheTLSSecretName,
								Namespace: "eclipse-che",
							},
						},
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      constants.DefaultProxyCredentialsSecret,
								Namespace: "eclipse-che",
							},
						},
						&corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Name:      constants.DefaultGitSelfSignedCertsConfigMapName,
								Namespace: "eclipse-che",
							},
						},
					).
					Build()
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
			})
		})

		Context("when a known object has other labels", func() {
			BeforeEach(func() {
				cheCtx = env.NewCheCtxBuilder().
					WithCheCluster(
						&chev2.CheCluster{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "eclipse-che",
								Namespace: "eclipse-che",
							},
						}).
					WithObjects(
						&corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      constants.SelfSignedCertificateSecretName,
								Namespace: "eclipse-che",
								Labels:    map[string]string{"custom": "value"},
							},
						},
					).
					Build()
			})

			It("should preserve existing labels when adding the part-of label", func() {
				_, done, err := reconciler.Reconcile(cheCtx)
				Expect(err).ToNot(HaveOccurred())
				Expect(done).To(BeTrue())

				key := types.NamespacedName{Namespace: "eclipse-che", Name: constants.SelfSignedCertificateSecretName}
				secret := &corev1.Secret{}
				Expect(env.Client.Get(env.Context, key, secret)).To(Succeed())
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
