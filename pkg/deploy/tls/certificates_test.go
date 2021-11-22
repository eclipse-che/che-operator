//
// Copyright (c) 2021 Red Hat, Inc.
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

	"testing"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestSyncTrustStoreConfigMapToCluster(t *testing.T) {
	checluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				ServerTrustStoreConfigMapName: "trust",
			},
		},
	}
	ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{})

	certificates := NewCertificatesReconciler()
	_, done, err := certificates.syncTrustStoreConfigMapToCluster(ctx)
	assert.Nil(t, err)
	assert.True(t, done)

	trustStoreConfigMap := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "trust", Namespace: "eclipse-che"}, trustStoreConfigMap)
	assert.Nil(t, err)
	assert.Equal(t, trustStoreConfigMap.ObjectMeta.Labels[injector], "true")
}

func TestSyncExistedTrustStoreConfigMapToCluster(t *testing.T) {
	trustStoreConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "trust",
			Namespace: "eclipse-che",
			Labels:    map[string]string{"a": "b"},
		},
		Data: map[string]string{"d": "c"},
	}
	checluster := &orgv1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "eclipse-che",
			Name:      "eclipse-che",
		},
		Spec: orgv1.CheClusterSpec{
			Server: orgv1.CheClusterSpecServer{
				ServerTrustStoreConfigMapName: "trust",
			},
		},
	}
	ctx := deploy.GetTestDeployContext(checluster, []runtime.Object{trustStoreConfigMap})

	certificates := NewCertificatesReconciler()
	_, done, err := certificates.syncTrustStoreConfigMapToCluster(ctx)
	assert.Nil(t, err)
	assert.True(t, done)

	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: "trust", Namespace: "eclipse-che"}, trustStoreConfigMap)
	assert.Nil(t, err)
	assert.Equal(t, trustStoreConfigMap.ObjectMeta.Labels[injector], "true")
	assert.Equal(t, trustStoreConfigMap.ObjectMeta.Labels["a"], "b")
	assert.Equal(t, trustStoreConfigMap.Data["d"], "c")
}

func TestSyncAdditionalCACertsConfigMapToCluster(t *testing.T) {
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
	cert2 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert2",
			Namespace: "eclipse-che",
			// Go client set up resource version 1 itself on object creation.
			// ResourceVersion: "1",
			Labels: map[string]string{
				"app.kubernetes.io/component": "ca-bundle",
				"app.kubernetes.io/part-of":   "che.eclipse.org"},
		},
		Data: map[string]string{"a2": "b2"},
	}

	ctx := deploy.GetTestDeployContext(nil, []runtime.Object{cert1})

	certificates := NewCertificatesReconciler()
	_, done, err := certificates.syncAdditionalCACertsConfigMapToCluster(ctx)
	assert.Nil(t, err)
	assert.True(t, done)

	cacertMerged := &corev1.ConfigMap{}
	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheAllCACertsConfigMapName, Namespace: "eclipse-che"}, cacertMerged)
	assert.Nil(t, err)
	assert.Equal(t, cacertMerged.ObjectMeta.Annotations["che.eclipse.org/included-configmaps"], "cert1-1")

	// let's create another configmap
	err = ctx.ClusterAPI.Client.Create(context.TODO(), cert2)
	assert.Nil(t, err)

	// check ca-cert-merged
	_, done, err = certificates.syncAdditionalCACertsConfigMapToCluster(ctx)
	assert.Nil(t, err)
	assert.False(t, done)

	_, done, err = certificates.syncAdditionalCACertsConfigMapToCluster(ctx)
	assert.Nil(t, err)
	assert.True(t, done)

	err = ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: CheAllCACertsConfigMapName, Namespace: "eclipse-che"}, cacertMerged)
	assert.Nil(t, err)
	assert.Equal(t, cacertMerged.ObjectMeta.Annotations["che.eclipse.org/included-configmaps"], "cert1-1.cert2-1")
}
