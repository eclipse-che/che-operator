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
	"context"
	"testing"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/deploy/metrics"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMigrateAddsPartOfLabelToTLSSecret(t *testing.T) {
	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultCheTLSSecretName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(tlsSecret).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultCheTLSSecretName, Namespace: "eclipse-che"}, secret)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
}

func TestMigrateAddsPartOfLabelToSelfSignedCertSecret(t *testing.T) {
	selfSignedCert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultSelfSignedCertificateSecretName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(selfSignedCert).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultSelfSignedCertificateSecretName, Namespace: "eclipse-che"}, secret)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
}

func TestMigrateAddsPartOfLabelToCaBundleConfigMap(t *testing.T) {
	caBundleCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultCaBundleCertsCMName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(caBundleCM).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	cm := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultCaBundleCertsCMName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
}

func TestMigrateAddsPartOfLabelToProxyCredentialsSecret(t *testing.T) {
	proxySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultProxyCredentialsSecret,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Components: chev2.CheClusterComponents{
				CheServer: chev2.CheServer{
					Proxy: &chev2.Proxy{},
				},
			},
		},
	}).WithObjects(proxySecret).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultProxyCredentialsSecret, Namespace: "eclipse-che"}, secret)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
}

func TestMigrateAddsPartOfLabelToGitTrustedCertsConfigMap(t *testing.T) {
	gitCertsCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultGitSelfSignedCertsConfigMapName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				TrustedCerts: &chev2.TrustedCerts{},
			},
		},
	}).WithObjects(gitCertsCM).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	cm := &corev1.ConfigMap{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: constants.DefaultGitSelfSignedCertsConfigMapName, Namespace: "eclipse-che"}, cm)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, cm.Labels[constants.KubernetesPartOfLabelKey])
}

func TestMigrateAddsPartOfLabelToServiceMonitors(t *testing.T) {
	cheServerServiceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metrics.CheServerServiceMonitorName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
	}
	dwoServiceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      metrics.DWOServiceMonitorName,
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
		},
	}

	ctx := test.NewCtxBuilder().WithObjects(cheServerServiceMonitor, dwoServiceMonitor).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	sm := &monitoringv1.ServiceMonitor{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: metrics.CheServerServiceMonitorName, Namespace: "eclipse-che"}, sm)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, sm.Labels[constants.KubernetesPartOfLabelKey])

	sm = &monitoringv1.ServiceMonitor{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: metrics.DWOServiceMonitorName, Namespace: "eclipse-che"}, sm)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, sm.Labels[constants.KubernetesPartOfLabelKey])
}

func TestMigrateWithCustomTLSSecretName(t *testing.T) {
	customTLSSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-tls-secret",
			Namespace: "eclipse-che",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}

	ctx := test.NewCtxBuilder().WithCheCluster(&chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				TlsSecretName: "custom-tls-secret",
			},
		},
	}).WithObjects(customTLSSecret).Build()

	migrator := NewMigrator()
	test.EnsureReconcile(t, ctx, migrator.Reconcile)

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "custom-tls-secret", Namespace: "eclipse-che"}, secret)
	assert.NoError(t, err)
	assert.Equal(t, constants.CheEclipseOrg, secret.Labels[constants.KubernetesPartOfLabelKey])
}
