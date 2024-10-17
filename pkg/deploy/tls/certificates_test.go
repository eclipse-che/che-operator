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
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/constants"

	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncOpenShiftCABundleCertificates(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

	certificates := NewCertificatesReconciler()

	done, err := certificates.syncOpenShiftCABundleCertificates(ctx)
	assert.Nil(t, err)
	assert.True(t, done)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, cm)
	assert.Nil(t, err)
	assert.Equal(t, "true", cm.ObjectMeta.Labels[injectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
}

func TestSyncExistedOpenShiftCABundleCertificates(t *testing.T) {
	openShiftCABundleCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ca-certs",
			Namespace: "eclipse-che",
			Labels:    map[string]string{"a": "b"},
		},
		Data: map[string]string{"d": "c"},
	}
	ctx := test.GetDeployContext(nil, []runtime.Object{openShiftCABundleCM})

	certificates := NewCertificatesReconciler()
	_, err := certificates.syncOpenShiftCABundleCertificates(ctx)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "ca-certs", Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, "true", cm.ObjectMeta.Labels[injectTrustedCaBundle])
	assert.Equal(t, constants.CheEclipseOrg, cm.ObjectMeta.Labels[constants.KubernetesPartOfLabelKey])
	assert.Equal(t, constants.CheCABundle, cm.ObjectMeta.Labels[constants.KubernetesComponentLabelKey])
	assert.Equal(t, "b", cm.ObjectMeta.Labels["a"])
	assert.Equal(t, "c", cm.Data["d"])
}

func TestSyncKubernetesCABundleCertificates(t *testing.T) {
	ctx := test.GetDeployContext(nil, []runtime.Object{})

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
	ctx := test.GetDeployContext(nil, []runtime.Object{kubeRootCert})

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
	ctx := test.GetDeployContext(cheCluster, []runtime.Object{gitCerts})

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
	ctx := test.GetDeployContext(nil, []runtime.Object{selfSignedCerts})

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
	ctx := test.GetDeployContext(nil, []runtime.Object{cert1})

	certificates := NewCertificatesReconciler()

	_, err := certificates.syncCheCABundleCerts(ctx)
	assert.Nil(t, err)

	cm := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.Nil(t, err)
	assert.Equal(t, "cert1#1 ", cm.ObjectMeta.Annotations["che.eclipse.org/included-configmaps"])

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
	assert.Equal(t, "cert1#1 cert2#1 ", cm.ObjectMeta.Annotations["che.eclipse.org/included-configmaps"])
	assert.Equal(t, cm.Data[kubernetesCABundleCertsFile], "# ConfigMap: cert1,  Key: a1\nb1\n\n# ConfigMap: cert2,  Key: a2\nb2\n\n")
}
