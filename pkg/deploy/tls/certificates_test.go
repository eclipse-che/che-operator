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
