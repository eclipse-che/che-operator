//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package org

import (
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"

	"github.com/che-incubator/kubernetes-image-puller-operator/api/v1alpha1"
	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

func TestV1ToV2alpha1(t *testing.T) {
	tolerations := []corev1.Toleration{
		{
			Key:      "a",
			Operator: corev1.TolerationOpEqual,
			Value:    "b",
		},
	}

	tolBytes, err := json.Marshal(tolerations)
	assert.NoError(t, err)

	tolerationStr := string(tolBytes)

	v1Obj := v1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "che-cluster",
			Annotations: map[string]string{
				"anno1": "annoValue1",
				"anno2": "annoValue2",
			},
		},
		Spec: v1.CheClusterSpec{
			Auth: v1.CheClusterSpecAuth{
				IdentityProviderURL: "kachny",
			},
			Database: v1.CheClusterSpecDB{
				ExternalDb:    true,
				PostgresImage: "postgres:the-best-version",
			},
			DevWorkspace: v1.CheClusterSpecDevWorkspace{},
			ImagePuller: v1.CheClusterSpecImagePuller{
				Spec: v1alpha1.KubernetesImagePullerSpec{
					ConfigMapName: "pulled-kachna",
				},
			},
			K8s: v1.CheClusterSpecK8SOnly{
				IngressDomain: "ingressDomain",
				IngressClass:  "traefik",
				TlsSecretName: "k8sSecret",
			},
			Metrics: v1.CheClusterSpecMetrics{
				Enable: true,
			},
			Server: v1.CheClusterSpecServer{
				CheHost:                "cheHost",
				CheImage:               "teh-che-severe",
				SingleHostGatewayImage: "single-host-image-of-the-year",
				CheHostTLSSecret:       "cheSecret",
				SingleHostGatewayConfigMapLabels: labels.Set{
					"a": "b",
					"c": "d",
				},
				CustomCheProperties: map[string]string{
					"CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX": "routeDomain",
					"CHE_WORKSPACE_POD_TOLERATIONS__JSON":           tolerationStr,
					"CHE_WORKSPACE_POD_NODE__SELECTOR":              "a=b,c=d",
				},
			},
			Storage: v1.CheClusterSpecStorage{
				PvcStrategy: "common",
			},
		},
	}

	t.Run("origInAnnos", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		err := V1ToV2alpha1(&v1Obj, v2)
		if err != nil {
			t.Error(err)
		}

		anno1 := v2.Annotations["anno1"]
		anno2 := v2.Annotations["anno2"]
		storedV1 := v2.Annotations[v1StorageAnnotation]

		if anno1 != "annoValue1" {
			t.Errorf("anno1 not copied")
		}

		if anno2 != "annoValue2" {
			t.Errorf("anno2 not copied")
		}

		if storedV1 == "" {
			t.Errorf("v2 should contain v1 data in annnotation")
		}

		restoredV1Spec := v1.CheClusterSpec{}
		if err = yaml.Unmarshal([]byte(storedV1), &restoredV1Spec); err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(&v1Obj.Spec, &restoredV1Spec) {
			t.Errorf("The spec should be restored verbatim from the annotations, but there's a diff %s", cmp.Diff(&v1Obj.Spec, &restoredV1Spec))
		}
	})

	t.Run("Enabled", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		err := V1ToV2alpha1(&v1Obj, v2)
		if err != nil {
			t.Error(err)
		}

		if *v2.Spec.Enabled {
			t.Errorf("Unexpected v2.Spec.Enabled: %s", v2.Spec.Gateway.Host)
		}
	})

	t.Run("Host-k8s", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		err := V1ToV2alpha1(&v1Obj, v2)
		if err != nil {
			t.Error(err)
		}

		if v2.Spec.Gateway.Host != "cheHost" {
			t.Errorf("Unexpected v2.Spec.Host: %s", v2.Spec.Gateway.Host)
		}
	})

	t.Run("WorkspaceDomainEndpointsBaseDomain-k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.Workspaces.DomainEndpoints.BaseDomain != "ingressDomain" {
				t.Errorf("Unexpected v2.Spec.Workspaces.DomainEndpoints.BaseDomain: %s", v2.Spec.Workspaces.DomainEndpoints.BaseDomain)
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsBaseDomain-opensfhit", func(t *testing.T) {
		onFakeOpenShift(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.Workspaces.DomainEndpoints.BaseDomain != "routeDomain" {
				t.Errorf("Unexpected v2.Spec.Workspaces.DomainEndpoints.BaseDomain: %s", v2.Spec.Workspaces.DomainEndpoints.BaseDomain)
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsTlsSecretName_k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.Workspaces.DomainEndpoints.TlsSecretName != "k8sSecret" {
				t.Errorf("Unexpected TlsSecretName")
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsTlsSecretName_OpenShift", func(t *testing.T) {
		onFakeOpenShift(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.Workspaces.DomainEndpoints.TlsSecretName != "" {
				t.Errorf("Unexpected TlsSecretName")
			}
		})
	})

	t.Run("GatewayEnabled", func(t *testing.T) {
		onFakeOpenShift(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.Gateway.Enabled == nil {
				t.Logf("The gateway.enabled attribute should be set explicitly after the conversion.")
				t.FailNow()
			}

			if !*v2.Spec.Gateway.Enabled {
				t.Errorf("The default for OpenShift is single")
			}
		})
	})

	t.Run("GatewayImage", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		err := V1ToV2alpha1(&v1Obj, v2)
		if err != nil {
			t.Error(err)
		}

		if v2.Spec.Gateway.Image != "single-host-image-of-the-year" {
			t.Errorf("Unexpected gateway image")
		}
	})

	t.Run("GatewayTlsSecretName", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		err := V1ToV2alpha1(&v1Obj, v2)
		if err != nil {
			t.Error(err)
		}

		if v2.Spec.Gateway.TlsSecretName != "cheSecret" {
			t.Errorf("Unexpected TlsSecretName")
		}
	})

	t.Run("GatewayConfigLabels", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		err := V1ToV2alpha1(&v1Obj, v2)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(v2.Spec.Gateway.ConfigLabels, v1Obj.Spec.Server.SingleHostGatewayConfigMapLabels) {
			t.Errorf("Unexpected Spec.Gateway.ConfigLabels: %v", cmp.Diff(v1Obj.Spec.Server.SingleHostGatewayConfigMapLabels, v2.Spec.Gateway.ConfigLabels))
		}
	})

	t.Run("WorkspacePodSelector", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		assert.NoError(t, V1ToV2alpha1(&v1Obj, v2))
		assert.Equal(t, map[string]string{"a": "b", "c": "d"}, v2.Spec.Workspaces.PodNodeSelector)
	})

	t.Run("WorkspacePodTolerations", func(t *testing.T) {
		v2 := &v2alpha1.CheCluster{}
		assert.NoError(t, V1ToV2alpha1(&v1Obj, v2))
		assert.Equal(t, tolerations, v2.Spec.Workspaces.PodTolerations)
	})
}

func TestV2alpha1ToV1(t *testing.T) {
	v2Obj := v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "che-cluster",
			Annotations: map[string]string{
				"anno1": "annoValue1",
				"anno2": "annoValue2",
			},
		},
		Spec: v2alpha1.CheClusterSpec{
			Enabled: pointer.BoolPtr(true),
			Workspaces: v2alpha1.Workspaces{
				DomainEndpoints: v2alpha1.DomainEndpoints{
					BaseDomain:    "baseDomain",
					TlsSecretName: "workspaceSecret",
				},
				PodNodeSelector: map[string]string{"a": "b", "c": "d"},
				PodTolerations: []corev1.Toleration{
					{
						Key:      "a",
						Operator: corev1.TolerationOpEqual,
						Value:    "b",
					},
				},
			},
			Gateway: v2alpha1.CheGatewaySpec{
				Host:            "v2Host",
				Enabled:         pointer.BoolPtr(true),
				Image:           "gateway-image",
				ConfigurerImage: "configurer-image",
				TlsSecretName:   "superSecret",
				ConfigLabels: labels.Set{
					"a": "b",
				},
			},
			K8s: v2alpha1.CheClusterSpecK8s{
				IngressAnnotations: map[string]string{
					"kubernetes.io/ingress.class": "some-other-ingress",
					"a":                           "b",
				},
			},
		},
	}

	t.Run("origInAnnos", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		err := V2alpha1ToV1(&v2Obj, v1)
		if err != nil {
			t.Error(err)
		}

		anno1 := v1.Annotations["anno1"]
		anno2 := v1.Annotations["anno2"]
		storedV2 := v1.Annotations[v2alpha1StorageAnnotation]

		if anno1 != "annoValue1" {
			t.Errorf("anno1 not copied")
		}

		if anno2 != "annoValue2" {
			t.Errorf("anno2 not copied")
		}

		if storedV2 == "" {
			t.Errorf("v1 should contain v2 data in annnotation")
		}

		restoredV2Spec := v2alpha1.CheClusterSpec{}
		yaml.Unmarshal([]byte(storedV2), &restoredV2Spec)

		if !reflect.DeepEqual(&v2Obj.Spec, &restoredV2Spec) {
			t.Errorf("The spec should be restored verbatim from the annotations, but there's a diff %s", cmp.Diff(&v2Obj.Spec, &restoredV2Spec))
		}
	})

	t.Run("Enabled", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		err := V2alpha1ToV1(&v2Obj, v1)
		if err != nil {
			t.Error(err)
		}

		if !v1.Spec.DevWorkspace.Enable {
			t.Errorf("Unexpected v1.Spec.DevWorkspace.Enable: %v", v1.Spec.DevWorkspace.Enable)
		}
	})

	t.Run("Host", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		err := V2alpha1ToV1(&v2Obj, v1)
		if err != nil {
			t.Error(err)
		}

		if v1.Spec.Server.CheHost != "v2Host" {
			t.Errorf("Unexpected v1.Spec.Server.CheHost: %s", v1.Spec.Server.CheHost)
		}
	})

	t.Run("WorkspaceDomainEndpointsBaseDomain-k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}

			if v1.Spec.K8s.IngressDomain != "baseDomain" {
				t.Errorf("Unexpected v1.Spec.K8s.IngressDomain: %s", v1.Spec.K8s.IngressDomain)
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsBaseDomain-openshift", func(t *testing.T) {
		onFakeOpenShift(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}

			if v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey] != "baseDomain" {
				t.Errorf("Unexpected v1.Spec.Server.CustomCheProperties[%s]: %s", routeDomainSuffixPropertyKey, v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey])
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsBaseDomain-openshift-should-not-be-set-empty-value", func(t *testing.T) {
		onFakeOpenShift(func() {
			v1 := &v1.CheCluster{}
			v2apha := v2Obj.DeepCopy()
			v2apha.Spec.Workspaces.DomainEndpoints.BaseDomain = ""
			err := V2alpha1ToV1(v2apha, v1)
			if err != nil {
				t.Error(err)
			}

			if _, ok := v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey]; ok {
				t.Errorf("Unexpected value. We shouldn't set key with empty value for %s custom Che property", routeDomainSuffixPropertyKey)
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsTlsSecretName_k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}

			if v1.Spec.K8s.TlsSecretName != "workspaceSecret" {
				t.Errorf("Unexpected TlsSecretName: %s", v1.Spec.K8s.TlsSecretName)
			}
		})
	})

	t.Run("WorkspaceDomainEndpointsTlsSecretName_OpenShift", func(t *testing.T) {
		onFakeOpenShift(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}

			if v1.Spec.K8s.TlsSecretName != "" {
				t.Errorf("Unexpected TlsSecretName")
			}
		})
	})

	t.Run("GatewayEnabled", func(t *testing.T) {
		onFakeOpenShift(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}
		})
	})

	t.Run("GatewayImage", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		err := V2alpha1ToV1(&v2Obj, v1)
		if err != nil {
			t.Error(err)
		}

		if v1.Spec.Server.SingleHostGatewayImage != "gateway-image" {
			t.Errorf("Unexpected gateway image")
		}
	})

	t.Run("GatewayTlsSecretName", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		err := V2alpha1ToV1(&v2Obj, v1)
		if err != nil {
			t.Error(err)
		}

		if v1.Spec.Server.CheHostTLSSecret != "superSecret" {
			t.Errorf("Unexpected TlsSecretName: %s", v1.Spec.Server.CheHostTLSSecret)
		}
	})

	t.Run("GatewayConfigLabels", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		err := V2alpha1ToV1(&v2Obj, v1)
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(v1.Spec.Server.SingleHostGatewayConfigMapLabels, v2Obj.Spec.Gateway.ConfigLabels) {
			t.Errorf("Unexpected SingleHostGatewayConfigMapLabels: %s", v1.Spec.Server.SingleHostGatewayConfigMapLabels)
		}
	})

	t.Run("WorkspacePodNodeSelector", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		assert.NoError(t, V2alpha1ToV1(&v2Obj, v1))
		assert.Equal(t, map[string]string{"a": "b", "c": "d"}, v1.Spec.Server.WorkspacePodNodeSelector)
	})

	t.Run("WorkspacePodTolerations", func(t *testing.T) {
		v1 := &v1.CheCluster{}
		assert.NoError(t, V2alpha1ToV1(&v2Obj, v1))
		assert.Equal(t, []corev1.Toleration{{
			Key:      "a",
			Operator: corev1.TolerationOpEqual,
			Value:    "b",
		}}, v1.Spec.Server.WorkspacePodTolerations)

	})
}

func TestFullCircleV1(t *testing.T) {
	v1Obj := v1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "che-cluster",
			Annotations: map[string]string{
				"anno1": "annoValue1",
				"anno2": "annoValue2",
			},
		},
		Spec: v1.CheClusterSpec{
			Auth: v1.CheClusterSpecAuth{
				IdentityProviderURL: "kachny",
			},
			Database: v1.CheClusterSpecDB{
				ExternalDb:    true,
				PostgresImage: "postgres:the-best-version",
			},
			DevWorkspace: v1.CheClusterSpecDevWorkspace{},
			ImagePuller: v1.CheClusterSpecImagePuller{
				Spec: v1alpha1.KubernetesImagePullerSpec{
					ConfigMapName: "pulled-kachna",
				},
			},
			K8s: v1.CheClusterSpecK8SOnly{
				IngressDomain: "ingressDomain",
				IngressClass:  "traefik",
				TlsSecretName: "k8sSecret",
			},
			Metrics: v1.CheClusterSpecMetrics{
				Enable: true,
			},
			Server: v1.CheClusterSpecServer{
				CheHost:                "cheHost",
				CheImage:               "teh-che-severe",
				SingleHostGatewayImage: "single-host-image-of-the-year",
				CheHostTLSSecret:       "cheSecret",
				SingleHostGatewayConfigMapLabels: labels.Set{
					"a": "b",
				},
				CustomCheProperties: map[string]string{
					"CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX": "routeDomain",
				},
			},
			Storage: v1.CheClusterSpecStorage{
				PvcStrategy: "common",
			},
		},
	}

	v2Obj := v2alpha1.CheCluster{}
	assert.NoError(t, V1ToV2alpha1(&v1Obj, &v2Obj))

	convertedV1 := v1.CheCluster{}
	assert.NoError(t, V2alpha1ToV1(&v2Obj, &convertedV1))

	assert.Empty(t, convertedV1.Annotations[v1StorageAnnotation])
	assert.NotEmpty(t, convertedV1.Annotations[v2alpha1StorageAnnotation])

	// remove v2 content annotation on the convertedV1 so that it doesn't interfere with the equality.
	delete(convertedV1.Annotations, v2alpha1StorageAnnotation)

	assert.Equal(t, &v1Obj, &convertedV1)
}

func TestFullCircleV2(t *testing.T) {
	v2Obj := v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "che-cluster",
			Annotations: map[string]string{
				"anno1": "annoValue1",
				"anno2": "annoValue2",
			},
		},
		Spec: v2alpha1.CheClusterSpec{
			Enabled: pointer.BoolPtr(true),
			Workspaces: v2alpha1.Workspaces{
				DomainEndpoints: v2alpha1.DomainEndpoints{
					BaseDomain:    "baseDomain",
					TlsSecretName: "workspaceSecret",
				},
			},
			Gateway: v2alpha1.CheGatewaySpec{
				Host:            "v2Host",
				Enabled:         pointer.BoolPtr(true),
				Image:           "gateway-image",
				ConfigurerImage: "configurer-image",
				TlsSecretName:   "superSecret",
				ConfigLabels: labels.Set{
					"a": "b",
				},
			},
			K8s: v2alpha1.CheClusterSpecK8s{
				IngressAnnotations: map[string]string{
					"kubernetes.io/ingress.class": "some-other-ingress",
					"a":                           "b",
				},
			},
		},
	}

	v1Obj := v1.CheCluster{}
	assert.NoError(t, V2alpha1ToV1(&v2Obj, &v1Obj))

	convertedV2 := v2alpha1.CheCluster{}
	assert.NoError(t, V1ToV2alpha1(&v1Obj, &convertedV2))

	assert.Empty(t, convertedV2.Annotations[v2alpha1StorageAnnotation])
	assert.NotEmpty(t, convertedV2.Annotations[v1StorageAnnotation])

	// remove v1 content annotation on the convertedV1 so that it doesn't interfere with the equality.
	delete(convertedV2.Annotations, v1StorageAnnotation)

	assert.Equal(t, &v2Obj, &convertedV2)
}

func onFakeOpenShift(f func()) {
	origOpenshift := util.IsOpenShift
	origOpenshift4 := util.IsOpenShift4

	util.IsOpenShift = true
	util.IsOpenShift4 = true

	f()

	util.IsOpenShift = origOpenshift
	util.IsOpenShift4 = origOpenshift4
}

func onFakeKubernetes(f func()) {
	origOpenshift := util.IsOpenShift
	origOpenshift4 := util.IsOpenShift4

	util.IsOpenShift = false
	util.IsOpenShift4 = false

	f()

	util.IsOpenShift = origOpenshift
	util.IsOpenShift4 = origOpenshift4
}
