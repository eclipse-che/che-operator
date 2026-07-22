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

package usernamespace

import (
	"context"
	"sync"
	"testing"

	"github.com/eclipse-che/che-operator/controllers/namespacecache"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"k8s.io/utils/ptr"

	"github.com/eclipse-che/che-operator/pkg/common/test"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	"github.com/eclipse-che/che-operator/pkg/deploy/tls"
	projectv1 "github.com/openshift/api/project/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func setupCheCluster(t *testing.T, ctx context.Context, cl client.Client, scheme *runtime.Scheme, cheNamespaceName string, cheName string) {
	var cheNamespace metav1.Object
	if infrastructure.IsOpenShift() {
		cheNamespace = &projectv1.Project{}
	} else {
		cheNamespace = &corev1.Namespace{}
	}

	cheNamespace.SetName(cheNamespaceName)
	if err := cl.Create(ctx, cheNamespace.(client.Object)); err != nil {
		t.Fatal(err)
	}

	cheCluster := chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cheName,
			Namespace: cheNamespaceName,
		},
		Spec: chev2.CheClusterSpec{
			DevEnvironments: chev2.CheClusterDevEnvironments{
				DisableContainerBuildCapabilities: ptr.To(true),
				NodeSelector:                      map[string]string{"a": "b", "c": "d"},
				Tolerations: []corev1.Toleration{
					{
						Key:      "a",
						Operator: corev1.TolerationOpEqual,
						Value:    "b",
					},
					{
						Key:      "c",
						Operator: corev1.TolerationOpEqual,
						Value:    "d",
					},
				},
				TrustedCerts: &chev2.TrustedCerts{
					GitTrustedCertsConfigMapName: "che-git-self-signed-cert",
				},
				SecondsOfInactivityBeforeIdling: ptr.To(int32(1800)),
				SecondsOfRunBeforeIdling:        ptr.To(int32(-1)),
				EditorsDownloadUrls: []chev2.EditorDownloadUrl{
					{
						Editor: "che-incubator/che-idea/latest",
						Url:    "url_latest",
					},
					{
						Editor: "che-incubator/che-idea/next",
						Url:    "url_next",
					},
				},
			},
			Networking: chev2.CheClusterSpecNetworking{
				Domain: "root-domain",
			},
		},
		Status: chev2.CheClusterStatus{
			CheURL: "https://che-host",
		},
	}
	if err := cl.Create(ctx, &cheCluster); err != nil {
		t.Fatal(err)
	}

	// also create the self-signed-certificate secret to pretend we have TLS set up
	cert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.DefaultSelfSignedCertificateSecretName,
			Namespace: cheNamespaceName,
		},
		Data: map[string][]byte{
			"ca.crt":     []byte("my certificate"),
			"other.data": []byte("should not be copied to target ns"),
		},
		Type:      "Opaque",
		Immutable: ptr.To(true),
	}
	if err := cl.Create(ctx, cert); err != nil {
		t.Fatal(err)
	}

	caCerts := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tls.CheMergedCABundleCertsCMName,
			Namespace: cheNamespaceName,
		},
		Data: map[string]string{
			"trusted1": "trusted cert 1",
			"trusted2": "trusted cert 2",
		},
	}
	if err := cl.Create(ctx, caCerts); err != nil {
		t.Fatal(err)
	}

	gitTlsCredentials := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-git-self-signed-cert",
			Namespace: cheNamespaceName,
		},
		Data: map[string]string{
			"githost": "the.host.of.git",
			"ca.crt":  "the public certificate of the.host.of.git",
		},
	}
	if err := cl.Create(ctx, gitTlsCredentials); err != nil {
		t.Fatal(err)
	}

}

func setup(infraType infrastructure.Type, objs ...client.Object) (*runtime.Scheme, client.Client, *CheUserNamespaceReconciler) {
	infrastructure.InitializeForTesting(infraType)

	ctx := test.NewCtxBuilder().WithObjects(objs...).WithCheCluster(nil).Build()

	cl := ctx.ClusterAPI.Client
	scheme := ctx.ClusterAPI.Scheme

	r := &CheUserNamespaceReconciler{
		client:                 cl,
		nonCachedClient:        cl,
		clientWrapper:          k8sclient.NewK8sClient(cl, scheme),
		nonCachedClientWrapper: k8sclient.NewK8sClient(cl, scheme),
		scheme:                 scheme,
		namespaceCache: &namespacecache.NamespaceCache{
			Client:          cl,
			KnownNamespaces: map[string]namespacecache.NamespaceInfo{},
			Lock:            sync.Mutex{},
		},
	}

	return scheme, cl, r
}
