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

package solver

import (
	"testing"

	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	controller "github.com/eclipse-che/che-operator/controllers/devworkspace"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func TestExternalTLSConfigForIngresses(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	mgr := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				TlsSecretName: "tlsSecret",
				Hostname:      "beyond.comprehension",
				Domain:        "almost.trivial",
				Annotations: map[string]string{
					"default_annotation_key": "default_annotation_value",
				},
			},
			DevEnvironments: chev2.CheClusterDevEnvironments{
				Networking: &chev2.DevEnvironmentNetworking{
					ExternalTLSConfig: &chev2.ExternalTLSConfig{
						Enabled: pointer.Bool(true),
						Annotations: map[string]string{
							"annotation_key": "annotation_value",
						},
						Labels: map[string]string{
							"label_key": "label_value",
						},
					},
				},
			},
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), userProfileSecret("username"),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tlsSecret",
				Namespace: "ns",
			},
			Data: map[string][]byte{
				"tls.key": []byte("asdf"),
				"tls.crt": []byte("qwer"),
			},
		})

	assert.Equal(t, 3, len(objs.Ingresses))

	// secure endpoint with custom TLS secret
	ingress := objs.Ingresses[0]
	assert.Equal(t, 1, len(ingress.Spec.TLS))
	assert.Equal(t, "username-my-workspace-e1.almost.trivial", ingress.Spec.TLS[0].Hosts[0])
	assert.Equal(t, "wsid-e1-tls", ingress.Spec.TLS[0].SecretName)
	assert.Equal(t, "annotation_value", ingress.Annotations["annotation_key"])
	assert.Equal(t, "label_value", ingress.Labels["label_key"])
	assert.NotContains(t, ingress.Annotations, "default_annotation_key")

	// secure endpoint, no custom TLS secret has been set so far
	ingress = objs.Ingresses[1]
	assert.Equal(t, 1, len(ingress.Spec.TLS))
	assert.Equal(t, "username-my-workspace-e2.almost.trivial", ingress.Spec.TLS[0].Hosts[0])
	assert.Equal(t, "wsid-e2-tls", ingress.Spec.TLS[0].SecretName)
	assert.Equal(t, "annotation_value", ingress.Annotations["annotation_key"])
	assert.Equal(t, "label_value", ingress.Labels["label_key"])
	assert.NotContains(t, ingress.Annotations, "default_annotation_key")

	// insecure endpoint
	ingress = objs.Ingresses[2]
	assert.Empty(t, ingress.Spec.TLS)
	assert.NotContains(t, ingress.Annotations, "annotation_key")
	assert.NotContains(t, ingress.Labels, "label_key")
	assert.Contains(t, ingress.Annotations, "default_annotation_key")
}

func TestExternalTLSConfigForRoutes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	mgr := &chev2.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{controller.FinalizerName},
		},
		Spec: chev2.CheClusterSpec{
			Networking: chev2.CheClusterSpecNetworking{
				Hostname:      "beyond.comprehension",
				TlsSecretName: "tlsSecret",
				Domain:        "almost.trivial",
				Labels: map[string]string{
					"default_label_key": "default_label_value",
				},
			},
			DevEnvironments: chev2.CheClusterDevEnvironments{
				Networking: &chev2.DevEnvironmentNetworking{
					ExternalTLSConfig: &chev2.ExternalTLSConfig{
						Enabled: pointer.Bool(true),
						Annotations: map[string]string{
							"annotation_key": "annotation_value",
						},
						Labels: map[string]string{
							"label_key": "label_value",
						},
					},
				},
			},
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), userProfileSecret("username"),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tlsSecret",
				Namespace: "ns",
			},
			Data: map[string][]byte{
				"tls.key": []byte("asdf"),
				"tls.crt": []byte("qwer"),
			},
		},
		&routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wsid-m1-9999-e1",
				Namespace: "ws",
			},
			Spec: routev1.RouteSpec{
				TLS: &routev1.TLSConfig{
					CACertificate: "certificate",
				},
			},
		})

	assert.Equal(t, 3, len(objs.Routes))

	// secure endpoint with custom TLS secret
	route := objs.Routes[0]
	assert.NotEmpty(t, route.Spec.TLS)
	assert.Equal(t, "certificate", route.Spec.TLS.CACertificate)
	assert.Equal(t, "annotation_value", route.Annotations["annotation_key"])
	assert.Equal(t, "label_value", route.Labels["label_key"])
	assert.NotContains(t, route.Labels, "default_label_key")

	// secure endpoint, no custom TLS secret has been set so far
	route = objs.Routes[1]
	assert.Empty(t, route.Spec.TLS)
	assert.Equal(t, "annotation_value", route.Annotations["annotation_key"])
	assert.Equal(t, "label_value", route.Labels["label_key"])
	assert.NotContains(t, route.Labels, "default_label_key")

	// insecure endpoint
	route = objs.Routes[2]
	assert.Empty(t, route.Spec.TLS)
	assert.NotContains(t, route.Annotations, "annotation_key")
	assert.NotContains(t, route.Labels, "label_key")
	assert.Contains(t, route.Labels, "default_label_key")
}
