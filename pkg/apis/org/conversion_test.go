package org

import (
	"reflect"
	"testing"

	"github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
	v1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/apis/org/v2alpha1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestV1ToV2alpha1(t *testing.T) {
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
				IngressDomain:   "ingressDomain",
				IngressClass:    "traefik",
				TlsSecretName:   "k8sSecret",
				IngressStrategy: "single-host",
			},
			Metrics: v1.CheClusterSpecMetrics{
				Enable: true,
			},
			Server: v1.CheClusterSpecServer{
				CheHost:                "cheHost",
				CheImage:               "teh-che-severe",
				SingleHostGatewayImage: "single-host-image-of-the-year",
				CustomCheProperties: map[string]string{
					"CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX": "routeDomain",
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

	t.Run("WorkspaceBaseDomain-k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.WorkspaceBaseDomain != "ingressDomain" {
				t.Errorf("Unexpected v2.Spec.WorkspaceBaseDomain: %s", v2.Spec.WorkspaceBaseDomain)
			}
		})
	})

	t.Run("WorkspaceBaseDomain-opensfhit", func(t *testing.T) {
		onFakeOpenShift(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.WorkspaceBaseDomain != "routeDomain" {
				t.Errorf("Unexpected v2.Spec.WorkspaceBaseDomain: %s", v2.Spec.WorkspaceBaseDomain)
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

			if *v2.Spec.Gateway.Enabled {
				t.Errorf("The default for OpenShift without devworkspace enabled (which is our testing object) is multihost, but we found v2 in singlehost.")
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

	t.Run("TlsSecretName_k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.TlsSecretName != "k8sSecret" {
				t.Errorf("Unexpected TlsSecretName")
			}
		})
	})

	t.Run("TlsSecretName_OpenShift", func(t *testing.T) {
		onFakeOpenShift(func() {
			v2 := &v2alpha1.CheCluster{}
			err := V1ToV2alpha1(&v1Obj, v2)
			if err != nil {
				t.Error(err)
			}

			if v2.Spec.TlsSecretName != "" {
				t.Errorf("Unexpected TlsSecretName")
			}
		})
	})
}

func TestV2alpha1ToV1(t *testing.T) {
	enabled := true
	v2Obj := v2alpha1.CheCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "che-cluster",
			Annotations: map[string]string{
				"anno1": "annoValue1",
				"anno2": "annoValue2",
			},
		},
		Spec: v2alpha1.CheClusterSpec{
			WorkspaceBaseDomain: "baseDomain",
			Gateway: v2alpha1.CheGatewaySpec{
				Host:            "v2Host",
				Enabled:         &enabled,
				Image:           "gateway-image",
				ConfigurerImage: "configurer-image",
			},
			TlsSecretName: "superSecret",
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

	t.Run("WorkspaceBaseDomain-k8s", func(t *testing.T) {
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

	t.Run("WorkspaceBaseDomain-openshift", func(t *testing.T) {
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

	t.Run("GatewayEnabled", func(t *testing.T) {
		onFakeOpenShift(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}

			if v1.Spec.Server.ServerExposureStrategy != "single-host" {
				t.Logf("When gateway.enabled is true in v2, v1 is single-host.")
				t.FailNow()
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

	t.Run("TlsSecretName_k8s", func(t *testing.T) {
		onFakeKubernetes(func() {
			v1 := &v1.CheCluster{}
			err := V2alpha1ToV1(&v2Obj, v1)
			if err != nil {
				t.Error(err)
			}

			if v1.Spec.K8s.TlsSecretName != "superSecret" {
				t.Errorf("Unexpected TlsSecretName: %s", v1.Spec.K8s.TlsSecretName)
			}
		})
	})

	t.Run("TlsSecretName_OpenShift", func(t *testing.T) {
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
