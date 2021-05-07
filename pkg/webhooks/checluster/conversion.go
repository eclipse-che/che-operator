package checluster

import (
	v1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/apis/org/v2alpha1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"gopkg.in/yaml.v2"
)

const (
	v1StorageAnnotation          = "che.eclipse.org/cheClusterV1Spec"
	v2alpha1StorageAnnotation    = "che.eclipse.org/cheClusterV2alpha1Spec"
	routeDomainSuffixPropertyKey = "CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX"
)

func V1ToV2alpha1(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) error {
	v2Data := v1.Annotations[v2alpha1StorageAnnotation]
	v2Spec := v2alpha1.CheClusterSpec{}
	if v2Data != "" {
		err := yaml.Unmarshal([]byte(v2Data), &v2Spec)
		if err != nil {
			return err
		}
	}

	v2.ObjectMeta = v1.ObjectMeta
	v2.Spec = v2Spec

	v1Spec, err := yaml.Marshal(v1.Spec)
	if err != nil {
		return err
	}
	if v2.Annotations == nil {
		v2.Annotations = map[string]string{}
	}
	v2.Annotations[v1StorageAnnotation] = string(v1Spec)

	v1ToV2alpha1_Host(v1, v2)
	v1ToV2alpha1_GatewayEnabled(v1, v2)
	v1ToV2alpha1_GatewayImage(v1, v2)
	v1ToV2alpha1_GatewayConfigurerImage(v1, v2)
	v1ToV2alpha1_TlsSecretName(v1, v2)
	v1ToV2alpha1_K8sIngressAnnotations(v1, v2)

	return nil
}

func V2alpha1ToV1(v2 *v2alpha1.CheCluster, v1Obj *v1.CheCluster) error {
	v1Data := v2.Annotations[v1StorageAnnotation]
	v1Spec := v1.CheClusterSpec{}
	if v1Data != "" {
		err := yaml.Unmarshal([]byte(v1Data), &v1Spec)
		if err != nil {
			return err
		}
	}

	v1Obj.ObjectMeta = v2.ObjectMeta
	v1Obj.Spec = v1Spec
	v1Obj.Status = v1.CheClusterStatus{}

	v2Spec, err := yaml.Marshal(v2.Spec)
	if err != nil {
		return err
	}
	if v1Obj.Annotations == nil {
		v1Obj.Annotations = map[string]string{}
	}
	v1Obj.Annotations[v2alpha1StorageAnnotation] = string(v2Spec)

	v2alpha1ToV1_Host(v1Obj, v2)
	v2alpha1ToV1_GatewayEnabled(v1Obj, v2)
	v2alpha1ToV1_GatewayImage(v1Obj, v2)
	v2alpha1ToV1_GatewayConfigurerImage(v1Obj, v2)
	v2alpha1ToV1_TlsSecretName(v1Obj, v2)
	v2alpha1ToV1_K8sIngressAnnotations(v1Obj, v2)

	return nil
}

func v1ToV2alpha1_Host(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	if util.IsOpenShift {
		v2.Spec.Host = v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey]
	} else {
		v2.Spec.Host = v1.Spec.K8s.IngressDomain
	}
}

func v1ToV2alpha1_GatewayEnabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	val := util.GetServerExposureStrategy(v1) == "single-host"
	v2.Spec.Gateway.Enabled = &val
}

func v1ToV2alpha1_GatewayImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.Image = v1.Spec.Server.SingleHostGatewayImage
}

func v1ToV2alpha1_GatewayConfigurerImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.ConfigurerImage = v1.Spec.Server.SingleHostGatewayConfigSidecarImage
}

func v1ToV2alpha1_TlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// v1.Spec.Server.CheHostTLSSecret is used specifically for Che Host - i.e. the host under which che server is deployed.
	// In DW we would only used that for subpath endpoints but wouldn't know what TLS to use for subdomain endpoints.
	// Che server always uses the default cluster certificate for these on OpenShift, and the K8s.TlsSecretName on K8s.
	// Because we're dealing with endpoints, let's try to use the secret on Kubernetes and nothing (e.g. the default cluster cert on OpenShift)
	// which is in line with the logic of the Che server. Let's ignore CheHostTLSSecret for now - this is used only for che server which
	// we are not exposing anyway.
	if !util.IsOpenShift {
		v2.Spec.TlsSecretName = v1.Spec.K8s.TlsSecretName
	}
}

func v1ToV2alpha1_K8sIngressAnnotations(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	annotations := map[string]string{
		"kubernetes.io/ingress.class":                       v1.Spec.K8s.IngressClass,
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
		"nginx.ingress.kubernetes.io/ssl-redirect":          "true",
	}
	// This is what is applied in the deploy/ingress.go but I don't think it is applicable in our situation
	// if ingressStrategy != "multi-host" && (component == DevfileRegistryName || component == PluginRegistryName) {
	// 	annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$1"
	// }

	v2.Spec.K8s.IngressAnnotations = annotations
}

func v2alpha1ToV1_Host(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	if util.IsOpenShift {
		if v1.Spec.Server.CustomCheProperties == nil {
			v1.Spec.Server.CustomCheProperties = map[string]string{}
		}
		v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey] = v2.Spec.Host
	} else {
		v1.Spec.K8s.IngressDomain = v2.Spec.Host
	}
}

func v2alpha1ToV1_GatewayEnabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1Strategy := util.GetServerExposureStrategy(v1)
	var v2Strategy string
	if *v2.Spec.Gateway.Enabled {
		v2Strategy = "single-host"
	} else {
		v2Strategy = "multi-host"
	}

	if v1Strategy != v2Strategy && v1Strategy != "default-host" {
		// we need to reconstruct what the configuration might have looked like in the original
		if v1.Spec.Server.ServerExposureStrategy == "" {
			if util.IsOpenShift {
				v1.Spec.Server.ServerExposureStrategy = v2Strategy
			} else {
				v1.Spec.K8s.IngressStrategy = v2Strategy
			}
		} else {
			v1.Spec.Server.ServerExposureStrategy = v2Strategy
		}
	}
}

func v2alpha1ToV1_GatewayImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.SingleHostGatewayImage = v2.Spec.Gateway.Image
}

func v2alpha1ToV1_GatewayConfigurerImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.SingleHostGatewayConfigSidecarImage = v2.Spec.Gateway.Image
}

func v2alpha1ToV1_TlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	if !util.IsOpenShift {
		v1.Spec.K8s.TlsSecretName = v2.Spec.TlsSecretName
	}
}

func v2alpha1ToV1_K8sIngressAnnotations(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	ingressClass := v2.Spec.K8s.IngressAnnotations["kubernetes.io/ingress.class"]
	if ingressClass == "" {
		ingressClass = "nginx"
	}
	v1.Spec.K8s.IngressClass = ingressClass
}
