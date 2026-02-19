//
// Copyright (c) 2019-2025 Red Hat, Inc.
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
	"context"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"k8s.io/utils/pointer"

	"github.com/eclipse-che/che-operator/pkg/common/constants"

	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncOpenShiftCABundleCertificates(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsCM := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, caCertsCM)
	assert.Nil(t, err)
	assert.Equal(t, "true", caCertsCM.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, caCertsCM.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, caCertsCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])

	caCertsMergedCM := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, kubernetesCABundleCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "subpath", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Empty(t, caCertsMergedCM.Data)
}

func TestSyncEmptyOpenShiftCABundleCertificates(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsCM := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, caCertsCM)
	assert.NoError(t, err)
	assert.Equal(t, "true", caCertsCM.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, caCertsCM.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, caCertsCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])

	// Let's pretend that OpenShift Network operator inject the CA bundle
	caCertsCM.Data = map[string]string{"ca-bundle.crt": "openshift-ca-bundle"}
	err = ctx.ClusterAPI.Client.Update(context.TODO(), caCertsCM)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsMergedCM := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, kubernetesCABundleCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "subpath", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Equal(t, caCertsMergedCM.Data["tls-ca-bundle.pem"], "# ConfigMap: ca-certs,  Key: ca-bundle.crt\nopenshift-ca-bundle\n\n")
}

func TestSyncOnlyCustomOpenShiftCertificates(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-openshift-trusted-certs-cm",
			Namespace: "openshift-config",
		},
		Data: map[string]string{
			"ca-bundle.crt": "openshift-cert",
		},
	}).Build()
	ctx.CheCluster.Spec.DevEnvironments.TrustedCerts = &chev2.TrustedCerts{DisableWorkspaceCaBundleMount: pointer.Bool(true)}
	ctx.Proxy.TrustedCAMapName = "custom-openshift-trusted-certs-cm"

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	cm := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, cm)
	assert.Nil(t, err)
	assert.Empty(t, cm.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "openshift-cert", cm.Data["ca-bundle.crt"])

	caCertsMergedCM := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.PublicCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "file", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Equal(t, caCertsMergedCM.Data["tls-ca-bundle.pem"], "# ConfigMap: ca-certs,  Key: ca-bundle.crt\nopenshift-cert\n\n")
}

func TestSyncKubernetesCABundleCertificates(t *testing.T) {
	ctx := test.NewCtxBuilder().Build()

	certificates := &CertificatesReconciler{
		readKubernetesCaBundle: func() ([]byte, error) {
			return []byte("kubernetes-ca-bundle"), nil
		},
	}

	done, err := certificates.syncKubernetesCABundleCertificates(ctx)
	assert.NoError(t, err)
	assert.True(t, done)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey], constants.CheEclipseOrg)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey], constants.CheCABundle)
}

func TestSyncKubernetesRootCertificates(t *testing.T) {
	kubeRootCert := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubernetesRootCACertsCMName,
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"ca.crt": "root-cert",
		},
	}
	ctx := test.NewCtxBuilder().WithObjects(kubeRootCert).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncKubernetesRootCertificates(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: kubernetesRootCACertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey], constants.CheEclipseOrg)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey], constants.CheCABundle)
}

func TestSyncGitTrustedCertificates(t *testing.T) {
	cheCluster := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "eclipse-che",
			Namespace: "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				TrustedCerts: &chev2.TrustedCerts{
					GitTrustedCertsConfigMapName: "git-trusted-certs",
				},
			},
		},
	}
	gitCerts := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-trusted-certs",
			Namespace: "eclipse-che",
		},
		Data: map[string]string{
			"ca.crt": "git-cert",
		},
	}
	ctx := test.NewCtxBuilder().WithCheCluster(cheCluster).WithObjects(gitCerts).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncGitTrustedCertificates(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "git-trusted-certs", Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey], constants.CheEclipseOrg)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey], constants.CheCABundle)
	assert.Equal(t, "git-cert", cm.Data["ca.crt"])
}

func TestSyncSelfSignedCertificates(t *testing.T) {
	selfSignedCerts := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultSelfSignedCertificateSecretName,
			Namespace: "eclipse-che",
		},
		Data: map[string][]byte{
			"ca.crt": []byte("self-signed-cert"),
		},
	}
	ctx := test.NewCtxBuilder().WithObjects(selfSignedCerts).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncSelfSignedCertificates(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultSelfSignedCertificateSecretName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey], constants.CheEclipseOrg)
	assert.Equal(t, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey], constants.CheCABundle)
	assert.Equal(t, "self-signed-cert", cm.Data["ca.crt"])
}

func TestSyncCheCABundleCerts(t *testing.T) {
	cert1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "cert1",
			Namespace:       "eclipse-che",
			ResourceVersion: "1",
			Labels: map[string]string{
				"app.kubernetes.io/component": "ca-bundle",
				"app.kubernetes.io/part-of":   "che.eclipse.org"},
		},
		Data: map[string]string{"a1": "b1"},
	}
	ctx := test.NewCtxBuilder().WithObjects(cert1).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.Nil(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.Nil(t, err)

	cert2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert2",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/component": "ca-bundle",
				"app.kubernetes.io/part-of":   "che.eclipse.org"},
		},
		Data: map[string]string{"a2": "b2"},
	}
	err = ctx.ClusterAPI.Client.Create(context.TODO(), cert2)
	assert.Nil(t, err)

	_, err = certificates.syncCheCABundleCerts(ctx)
	assert.Nil(t, err)

	cm = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.Nil(t, err)
	assert.Equal(t, cm.Data[kubernetesCABundleCertsFile], "# ConfigMap: cert1,  Key: a1\nb1\n\n# ConfigMap: cert2,  Key: a2\nb2\n\n")
}

func TestSyncCheCABundleCertsDeterministicKeyOrder(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert1",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"app.kubernetes.io/component": "ca-bundle",
					"app.kubernetes.io/part-of":   "che.eclipse.org",
				},
			},
			Data: map[string]string{
				"z-key": "z-value",
				"a-key": "a-value",
				"m-key": "m-value",
			},
		}).Build()

	certificatesReconciler := NewCertificatesReconciler()

	_, err := certificatesReconciler.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.Nil(t, err)

	expected := "# ConfigMap: cert1,  Key: a-key\na-value\n\n" +
		"# ConfigMap: cert1,  Key: m-key\nm-value\n\n" +
		"# ConfigMap: cert1,  Key: z-key\nz-value\n\n"
	assert.Equal(t, expected, cm.Data[kubernetesCABundleCertsFile])
}

func TestSyncCheCABundleCertsGitTrustedCertsExcludesGitHostKey(t *testing.T) {
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				DevEnvironments: chev2.CheClusterDevEnvironments{
					TrustedCerts: &chev2.TrustedCerts{
						GitTrustedCertsConfigMapName: "git-trusted-certs",
					},
				},
			},
		}).WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-trusted-certs",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"app.kubernetes.io/component": constants.CheCABundle,
					"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
				},
			},
			Data: map[string]string{
				constants.GitSelfSignedCertsConfigMapCertKey:    "git-cert-value",
				constants.GitSelfSignedCertsConfigMapGitHostKey: "https://git.example.com",
			},
		},
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-cert",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"app.kubernetes.io/component": constants.CheCABundle,
					"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
				},
			},
			Data: map[string]string{
				"cert-key": "other-cert-value",
			},
		}).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)

	// Verify that the githost key is excluded from the merged bundle
	expected := "# ConfigMap: git-trusted-certs,  Key: ca.crt\ngit-cert-value\n\n" +
		"# ConfigMap: other-cert,  Key: cert-key\nother-cert-value\n\n"

	assert.Equal(t, expected, cm.Data[kubernetesCABundleCertsFile])
}

func TestSyncCheCABundleCertsExcludesGitHostKeyWithDefaultConfigMap(t *testing.T) {
	ctx := test.NewCtxBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.DefaultGitSelfSignedCertsConfigMapName, // "che-git-self-signed-cert"
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"app.kubernetes.io/component": constants.CheCABundle,
					"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
				},
			},
			Data: map[string]string{
				constants.GitSelfSignedCertsConfigMapCertKey:    "cert-data",
				constants.GitSelfSignedCertsConfigMapGitHostKey: "https://git.example.com",
			},
		}).Build()

	certificates := NewCertificatesReconciler()
	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)

	// Verify githost is excluded even when using default ConfigMap name
	expected := "# ConfigMap: che-git-self-signed-cert,  Key: ca.crt\ncert-data\n\n"
	assert.Equal(t, expected, cm.Data[kubernetesCABundleCertsFile])
}

func TestToggleDisableWorkspaceCaBundleMount(t *testing.T) {
	// Enable workspace CA bundle mount
	ctx := test.NewCtxBuilder().WithObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-openshift-trusted-certs-cm",
			Namespace: "openshift-config",
		},
		Data: map[string]string{
			"ca-bundle.crt": "openshift-cert",
		},
	}).Build()
	ctx.Proxy.TrustedCAMapName = "custom-openshift-trusted-certs-cm"
	ctx.CheCluster.Spec.DevEnvironments.TrustedCerts = &chev2.TrustedCerts{DisableWorkspaceCaBundleMount: pointer.Bool(false)}

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsCM := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, caCertsCM)
	assert.Nil(t, err)
	assert.Equal(t, "true", caCertsCM.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, caCertsCM.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, caCertsCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])

	// Let's pretend that OpenShift Network operator inject the CA bundle
	caCertsCM.Data = map[string]string{"ca-bundle.crt": "openshift-ca-bundle"}
	err = ctx.ClusterAPI.Client.Update(context.TODO(), caCertsCM)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsMergedCM := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, kubernetesCABundleCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "0444", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation])
	assert.Equal(t, "subpath", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Equal(t, caCertsMergedCM.Data["tls-ca-bundle.pem"], "# ConfigMap: ca-certs,  Key: ca-bundle.crt\nopenshift-ca-bundle\n\n")
	assert.Equal(t, 1, len(caCertsMergedCM.Data))

	// Disable workspace CA bundle mount
	ctx.CheCluster.Spec.DevEnvironments.TrustedCerts = &chev2.TrustedCerts{DisableWorkspaceCaBundleMount: pointer.Bool(true)}

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, caCertsCM)
	assert.Nil(t, err)
	assert.Empty(t, caCertsCM.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, caCertsCM.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, caCertsCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "openshift-cert", caCertsCM.Data["ca-bundle.crt"])

	caCertsMergedCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, constants.PublicCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "0444", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation])
	assert.Equal(t, "file", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Equal(t, caCertsMergedCM.Data["tls-ca-bundle.pem"], "# ConfigMap: ca-certs,  Key: ca-bundle.crt\nopenshift-cert\n\n")
	assert.Equal(t, 1, len(caCertsMergedCM.Data))

	// Enable workspace CA bundle mount
	ctx.CheCluster.Spec.DevEnvironments.TrustedCerts = &chev2.TrustedCerts{DisableWorkspaceCaBundleMount: pointer.Bool(false)}
	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, caCertsCM)
	assert.Nil(t, err)
	assert.Equal(t, "true", caCertsCM.ObjectMeta.Labels[constants.ConfigOpenShiftIOInjectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, caCertsCM.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, caCertsCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])

	// Let's pretend that OpenShift Network operator inject the CA bundle
	caCertsCM.Data = map[string]string{"ca-bundle.crt": "openshift-ca-bundle-new"}
	err = ctx.ClusterAPI.Client.Update(context.TODO(), caCertsCM)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsMergedCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, kubernetesCABundleCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "0444", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation])
	assert.Equal(t, "subpath", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Equal(t, caCertsMergedCM.Data["tls-ca-bundle.pem"], "# ConfigMap: ca-certs,  Key: ca-bundle.crt\nopenshift-ca-bundle-new\n\n")
	assert.Equal(t, 1, len(caCertsMergedCM.Data))

	// Check CM is reverted after changing the annotations
	caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation] = "a"
	caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation] = "b"
	err = ctx.ClusterAPI.Client.Update(context.TODO(), caCertsMergedCM)
	assert.NoError(t, err)

	test.EnsureReconcile(t, ctx, NewCertificatesReconciler().Reconcile)

	caCertsMergedCM = &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs-merged", Namespace: "eclipse-che"}, caCertsMergedCM)
	assert.Nil(t, err)
	assert.Equal(t, constants.WorkspacesConfig, caCertsMergedCM.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, kubernetesCABundleCertsDir, caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountPathAnnotation])
	assert.Equal(t, "0444", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAccessModeAnnotation])
	assert.Equal(t, "subpath", caCertsMergedCM.ObjectMeta.Annotations[dwconstants.DevWorkspaceMountAsAnnotation])
	assert.Equal(t, caCertsMergedCM.Data["tls-ca-bundle.pem"], "# ConfigMap: ca-certs,  Key: ca-bundle.crt\nopenshift-ca-bundle-new\n\n")
	assert.Equal(t, 1, len(caCertsMergedCM.Data))
}

func TestSyncCheCABundleCertsWithEmptyConfigMap(t *testing.T) {
	// A CA bundle ConfigMap exists but has no data entries
	emptyCert := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-cert",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/component": constants.CheCABundle,
				"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
			},
		},
		Data: map[string]string{},
	}
	ctx := test.NewCtxBuilder().WithObjects(emptyCert).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)

	// Merged CM should have no tls-ca-bundle.pem key when source ConfigMap is empty
	assert.Empty(t, cm.Data)
}

func TestSyncCheCABundleCertsWithEmptyAndNonEmptyConfigMaps(t *testing.T) {
	emptyCert := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-cert",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/component": constants.CheCABundle,
				"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
			},
		},
		Data: map[string]string{},
	}
	nonEmptyCert := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-empty-cert",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/component": constants.CheCABundle,
				"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
			},
		},
		Data: map[string]string{"ca.crt": "some-cert"},
	}
	ctx := test.NewCtxBuilder().WithObjects(emptyCert, nonEmptyCert).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)

	// Only the non-empty ConfigMap's cert should be in the merged bundle
	expected := "# ConfigMap: non-empty-cert,  Key: ca.crt\nsome-cert\n\n"
	assert.Equal(t, expected, cm.Data[kubernetesCABundleCertsFile])
}

func TestSyncCheCABundleCertsWithNilDataConfigMap(t *testing.T) {
	// A CA bundle ConfigMap exists with nil Data (not initialized)
	nilDataCert := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nil-data-cert",
			Namespace: "eclipse-che",
			Labels: map[string]string{
				"app.kubernetes.io/component": constants.CheCABundle,
				"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
			},
		},
	}
	ctx := test.NewCtxBuilder().WithObjects(nilDataCert).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)

	// Merged CM should have no tls-ca-bundle.pem key when source ConfigMap has nil Data
	assert.Empty(t, cm.Data)
}

func TestSyncCheCABundleCertsGitTrustedCertsOnlyGitHostKey(t *testing.T) {
	// Git trusted certs ConfigMap has only the githost key (no actual cert)
	ctx := test.NewCtxBuilder().WithCheCluster(
		&chev2.CheCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eclipse-che",
				Namespace: "eclipse-che",
			},
			Spec: chev2.CheClusterSpec{
				DevEnvironments: chev2.CheClusterDevEnvironments{
					TrustedCerts: &chev2.TrustedCerts{
						GitTrustedCertsConfigMapName: "git-trusted-certs",
					},
				},
			},
		}).WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-trusted-certs",
				Namespace: "eclipse-che",
				Labels: map[string]string{
					"app.kubernetes.io/component": constants.CheCABundle,
					"app.kubernetes.io/part-of":   constants.CheEclipseOrg,
				},
			},
			Data: map[string]string{
				constants.GitSelfSignedCertsConfigMapGitHostKey: "https://git.example.com",
			},
		}).Build()

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)

	// All keys were skipped (githost is excluded), so merged CM should be empty
	assert.Empty(t, cm.Data)
}
