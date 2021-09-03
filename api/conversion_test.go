package org

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/che-incubator/kubernetes-image-puller-operator/pkg/apis/che/v1alpha1"
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
				CheHostTLSSecret:       "cheSecret",
				SingleHostGatewayConfigMapLabels: labels.Set{
					"a": "b",
					"c": "d",
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

			if v2.Spec.WorkspaceDomainEndpoints.BaseDomain != "ingressDomain" {
				t.Errorf("Unexpected v2.Spec.WorkspaceDomainEndpoints.BaseDomain: %s", v2.Spec.WorkspaceDomainEndpoints.BaseDomain)
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

			if v2.Spec.WorkspaceDomainEndpoints.BaseDomain != "routeDomain" {
				t.Errorf("Unexpected v2.Spec.WorkspaceWorkspaceDomainEndpoints.BaseDomainBaseDomain: %s", v2.Spec.WorkspaceDomainEndpoints.BaseDomain)
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

			if v2.Spec.WorkspaceDomainEndpoints.TlsSecretName != "k8sSecret" {
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

			if v2.Spec.WorkspaceDomainEndpoints.TlsSecretName != "" {
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
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain:    "baseDomain",
				TlsSecretName: "workspaceSecret",
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
			v2apha.Spec.WorkspaceDomainEndpoints.BaseDomain = ""
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

			if util.GetServerExposureStrategy(v1) != "single-host" {
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
}

func TestExposureStrategyConversions(t *testing.T) {
	testWithExposure := func(v1ExposureStrategy string, v1IngressStrategy string, v1DevWorkspaceEnabled bool, v2GatewayEnabledChange *bool, test func(*testing.T, *v1.CheCluster)) {
		origV1 := &v1.CheCluster{
			Spec: v1.CheClusterSpec{
				Server: v1.CheClusterSpecServer{
					ServerExposureStrategy: v1ExposureStrategy,
				},
				K8s: v1.CheClusterSpecK8SOnly{
					IngressStrategy: v1IngressStrategy,
				},
				DevWorkspace: v1.CheClusterSpecDevWorkspace{
					Enable: v1DevWorkspaceEnabled,
				},
			},
		}

		t.Run(fmt.Sprintf("[v1ExposureStrategy=%v/v1IngressStrategy=%v/v1DevworkspaceEnabled=%v/v2GatewayEnabledChange=%v]", v1ExposureStrategy, v1IngressStrategy, v1DevWorkspaceEnabled, v2GatewayEnabledChange), func(t *testing.T) {
			v2 := &v2alpha1.CheCluster{}
			if err := V1ToV2alpha1(origV1, v2); err != nil {
				t.Error(err)
			}

			if v2GatewayEnabledChange != nil {
				v2.Spec.Gateway.Enabled = v2GatewayEnabledChange
			}

			// now convert back and run the test
			v1Tested := &v1.CheCluster{}
			if err := V2alpha1ToV1(v2, v1Tested); err != nil {
				t.Error(err)
			}

			test(t, v1Tested)
		})
	}

	testWithExposure("single-host", "", true, nil, func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "single-host" {
			t.Errorf("The v1 should have single-host exposure after conversion")
		}
	})

	testWithExposure("single-host", "", true, pointer.BoolPtr(false), func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "multi-host" {
			t.Errorf("The v1 should have multi-host exposure after conversion")
		}
	})

	testWithExposure("multi-host", "", true, nil, func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "multi-host" {
			t.Errorf("The v1 should have multi-host exposure after conversion")
		}
	})

	testWithExposure("multi-host", "", true, pointer.BoolPtr(true), func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "single-host" {
			t.Errorf("The v1 should have single-host exposure after conversion")
		}
	})

	testWithExposure("default-host", "", true, nil, func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "default-host" {
			t.Errorf("The v1 should have default-host exposure after conversion")
		}
	})

	testWithExposure("default-host", "", true, pointer.BoolPtr(true), func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "default-host" {
			t.Errorf("The v1 should have default-host exposure after conversion")
		}
	})

	testWithExposure("default-host", "", true, pointer.BoolPtr(false), func(t *testing.T, old *v1.CheCluster) {
		if old.Spec.Server.ServerExposureStrategy != "multi-host" {
			t.Errorf("The v1 should have multi-host exposure after conversion")
		}
	})

	onFakeKubernetes(func() {
		testWithExposure("", "single-host", true, nil, func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "single-host" {
				t.Errorf("The ingress strategy should have been unchanged after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})

		testWithExposure("", "single-host", true, pointer.BoolPtr(false), func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "multi-host" {
				t.Errorf("The ingress strategy should have been set to multi-host after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})

		testWithExposure("", "single-host", true, pointer.BoolPtr(true), func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "single-host" {
				t.Errorf("The ingress strategy should have been unchanged after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})

		testWithExposure("", "multi-host", true, nil, func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "multi-host" {
				t.Errorf("The ingress strategy should have been unchanged after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})

		// the below two tests test that we're leaving the ingress strategy unchanged if it doesn't affect the effective exposure
		// strategy
		testWithExposure("", "multi-host", true, pointer.BoolPtr(false), func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "multi-host" {
				t.Errorf("The ingress strategy should have been unchanged after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})

		testWithExposure("", "multi-host", true, pointer.BoolPtr(true), func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "single-host" {
				t.Errorf("The ingress strategy should have been unchanged after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})

		testWithExposure("", "default-host", true, nil, func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
			if old.Spec.K8s.IngressStrategy != "default-host" {
				t.Errorf("The ingress strategy should have been unchanged after conversion but was: %v", old.Spec.K8s.IngressStrategy)
			}
		})
	})

	onFakeOpenShift(func() {
		testWithExposure("", "", true, nil, func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
		})

		testWithExposure("", "", false, nil, func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
		})

		testWithExposure("", "", true, pointer.BoolPtr(false), func(t *testing.T, old *v1.CheCluster) {
			// default on openshift with devworkspace enabled in v1 is single-host, but we've disabled the gateway in v2. So after the conversion
			// v1 should change to an explicit multi-host.
			if old.Spec.Server.ServerExposureStrategy != "multi-host" {
				t.Errorf("The server exposure strategy should have been set to multi-host after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
		})

		testWithExposure("", "", true, pointer.BoolPtr(true), func(t *testing.T, old *v1.CheCluster) {
			if old.Spec.Server.ServerExposureStrategy != "" {
				t.Errorf("The server exposure strategy should have been left empty after conversion but was: %v", old.Spec.Server.ServerExposureStrategy)
			}
		})
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
	V1ToV2alpha1(&v1Obj, &v2Obj)

	convertedV1 := v1.CheCluster{}
	V2alpha1ToV1(&v2Obj, &convertedV1)

	if !reflect.DeepEqual(&v1Obj, &convertedV1) {
		t.Errorf("V1 not equal to itself after the conversion through v2alpha1: %v", cmp.Diff(&v1Obj, &convertedV1))
	}

	if convertedV1.Annotations[v1StorageAnnotation] != "" {
		t.Errorf("The v1 storage annotations should not be present on the v1 object")
	}

	if convertedV1.Annotations[v2alpha1StorageAnnotation] == "" {
		t.Errorf("The v2alpha1 storage annotation should be present on the v1 object")
	}
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
			WorkspaceDomainEndpoints: v2alpha1.WorkspaceDomainEndpoints{
				BaseDomain:    "baseDomain",
				TlsSecretName: "workspaceSecret",
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
	V2alpha1ToV1(&v2Obj, &v1Obj)

	convertedV2 := v2alpha1.CheCluster{}
	V1ToV2alpha1(&v1Obj, &convertedV2)

	if !reflect.DeepEqual(&v2Obj, &convertedV2) {
		t.Errorf("V2alpha1 not equal to itself after the conversion through v1: %v", cmp.Diff(&v2Obj, &convertedV2))
	}

	if convertedV2.Annotations[v2alpha1StorageAnnotation] != "" {
		t.Errorf("The v2alpha1 storage annotations should not be present on the v2alpha1 object")
	}

	if convertedV2.Annotations[v1StorageAnnotation] == "" {
		t.Errorf("The v1 storage annotation should be present on the v2alpha1 object")
	}
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
