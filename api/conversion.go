package org

import (
	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"sigs.k8s.io/yaml"
)

const (
	v1StorageAnnotation          = "che.eclipse.org/cheClusterV1Spec"
	v2alpha1StorageAnnotation    = "che.eclipse.org/cheClusterV2alpha1Spec"
	routeDomainSuffixPropertyKey = "CHE_INFRA_OPENSHIFT_ROUTE_HOST_DOMAIN__SUFFIX"
	defaultV2alpha1IngressClass  = "nginx"
	defaultV1IngressClass        = "nginx"
)

func AsV1(v2 *v2alpha1.CheCluster) *v1.CheCluster {
	ret := &v1.CheCluster{}
	V2alpha1ToV1(v2, ret)
	return ret
}

func AsV2alpha1(v1 *v1.CheCluster) *v2alpha1.CheCluster {
	ret := &v2alpha1.CheCluster{}
	V1ToV2alpha1(v1, ret)
	return ret
}

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

	v1ToV2alpha1_Enabled(v1, v2)
	v1ToV2alpha1_Host(v1, v2)
	v1ToV2alpha1_GatewayEnabled(v1, v2)
	v1ToV2alpha1_GatewayImage(v1, v2)
	v1ToV2alpha1_GatewayConfigurerImage(v1, v2)
	v1ToV2alpha1_GatewayTlsSecretName(v1, v2)
	v1toV2alpha1_GatewayConfigLabels(v1, v2)
	v1ToV2alpha1_WorkspaceDomainEndpointsBaseDomain(v1, v2)
	v1ToV2alpha1_WorkspaceDomainEndpointsTlsSecretName(v1, v2)
	v1ToV2alpha1_K8sIngressAnnotations(v1, v2)

	// we don't need to store the serialized v2 on a v2 object
	delete(v2.Annotations, v2alpha1StorageAnnotation)

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

	v2alpha1ToV1_Enabled(v1Obj, v2)
	v2alpha1ToV1_Host(v1Obj, v2)
	v2alpha1ToV1_GatewayEnabled(v1Obj, v2)
	v2alpha1ToV1_GatewayImage(v1Obj, v2)
	v2alpha1ToV1_GatewayConfigurerImage(v1Obj, v2)
	v2alpha1ToV1_GatewayTlsSecretName(v1Obj, v2)
	v2alpha1ToV1_GatewayConfigLabels(v1Obj, v2)
	v2alpha1ToV1_WorkspaceDomainEndpointsBaseDomain(v1Obj, v2)
	v2alpha1ToV1_WorkspaceDomainEndpointsTlsSecretName(v1Obj, v2)
	v2alpha1ToV1_K8sIngressAnnotations(v1Obj, v2)

	// we don't need to store the serialized v1 on a v1 object
	delete(v1Obj.Annotations, v1StorageAnnotation)

	return nil
}

func v1ToV2alpha1_Enabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Enabled = &v1.Spec.DevWorkspace.Enable
}

func v1ToV2alpha1_Host(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.Host = v1.Spec.Server.CheHost
}

func v1ToV2alpha1_WorkspaceDomainEndpointsBaseDomain(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	if util.IsOpenShift {
		v2.Spec.WorkspaceDomainEndpoints.BaseDomain = v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey]
	} else {
		v2.Spec.WorkspaceDomainEndpoints.BaseDomain = v1.Spec.K8s.IngressDomain
	}
}

func v1ToV2alpha1_WorkspaceDomainEndpointsTlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// Che server always uses the default cluster certificate for subdomain workspace endpoints on OpenShift, and the K8s.TlsSecretName on K8s.
	// Because we're dealing with endpoints, let's try to use the secret on Kubernetes and nothing (e.g. the default cluster cert on OpenShift)
	// which is in line with the logic of the Che server.
	if !util.IsOpenShift {
		v2.Spec.WorkspaceDomainEndpoints.TlsSecretName = v1.Spec.K8s.TlsSecretName
	}
}

func v1ToV2alpha1_GatewayEnabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	exposureStrategy := util.GetServerExposureStrategy(v1)
	// On Kubernetes, we can have single-host realized using ingresses (that use the same host but different paths).
	// This is actually not supported on DWCO where we always use the gateway for that. So here, we actually just
	// ignore the Spec.K8s.SingleHostExposureType, but we need to be aware of it when converting back.
	// Note that default-host is actually not supported on v2, but it is similar enough to default host that we
	// treat it as such for v2. The difference between default-host and single-host is that the default-host uses
	// the cluster domain itself as the base domain whereas single-host uses a configured domain. In v2 we always
	// need a domain configured.
	val := exposureStrategy == "single-host" || exposureStrategy == "default-host"
	v2.Spec.Gateway.Enabled = &val
}

func v1ToV2alpha1_GatewayImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.Image = v1.Spec.Server.SingleHostGatewayImage
}

func v1ToV2alpha1_GatewayConfigurerImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.ConfigurerImage = v1.Spec.Server.SingleHostGatewayConfigSidecarImage
}

func v1ToV2alpha1_GatewayTlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// v1.Spec.Server.CheHostTLSSecret is used specifically for Che Host - i.e. the host under which che server is deployed.
	// In DW we would only used that for subpath endpoints but wouldn't know what TLS to use for subdomain endpoints.

	v2.Spec.Gateway.TlsSecretName = v1.Spec.Server.CheHostTLSSecret
}

func v1toV2alpha1_GatewayConfigLabels(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.ConfigLabels = v1.Spec.Server.SingleHostGatewayConfigMapLabels
}

func v1ToV2alpha1_K8sIngressAnnotations(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// The only property in v1 spec that boils down to the ingress annotations is the K8s.IngressClass
	if v1.Spec.K8s.IngressClass != "" && v1.Spec.K8s.IngressClass != defaultV2alpha1IngressClass {
		if v2.Spec.K8s.IngressAnnotations == nil {
			v2.Spec.K8s.IngressAnnotations = map[string]string{}
		}
		v2.Spec.K8s.IngressAnnotations["kubernetes.io/ingress.class"] = v1.Spec.K8s.IngressClass
	}

	// This is what is applied in the deploy/ingress.go but I don't think it is applicable in our situation
	// if ingressStrategy != "multi-host" && (component == DevfileRegistryName || component == PluginRegistryName) {
	// 	annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$1"
	// }
}

func v2alpha1ToV1_Enabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.DevWorkspace.Enable = v2.Spec.IsEnabled()
}

func v2alpha1ToV1_Host(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.CheHost = v2.Spec.Gateway.Host
}

func v2alpha1ToV1_WorkspaceDomainEndpointsBaseDomain(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	if util.IsOpenShift {
		if v1.Spec.Server.CustomCheProperties == nil {
			v1.Spec.Server.CustomCheProperties = map[string]string{}
		}
		if len(v2.Spec.WorkspaceDomainEndpoints.BaseDomain) > 0 {
			v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey] = v2.Spec.WorkspaceDomainEndpoints.BaseDomain
		}
	} else {
		v1.Spec.K8s.IngressDomain = v2.Spec.WorkspaceDomainEndpoints.BaseDomain
	}
}

func v2alpha1ToV1_WorkspaceDomainEndpointsTlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// see the comments in the v1 to v2alpha1 conversion method
	if !util.IsOpenShift {
		v1.Spec.K8s.TlsSecretName = v2.Spec.WorkspaceDomainEndpoints.TlsSecretName
	}
}

func v2alpha1ToV1_GatewayEnabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1Strategy := util.GetServerExposureStrategy(v1)
	v1IngressStrategy := v1.Spec.K8s.IngressStrategy

	var v2Strategy string
	if v2.Spec.Gateway.IsEnabled() {
		v2Strategy = "single-host"
	} else {
		v2Strategy = "multi-host"
	}

	if v1.Spec.Server.ServerExposureStrategy == "" {
		// in the original, the server exposure strategy was undefined, so we need to check whether we can leave it that way
		if util.IsOpenShift {
			if v2Strategy != v1Strategy {
				// only update if the v2Strategy doesn't correspond to the default determined from the v1
				v1.Spec.Server.ServerExposureStrategy = v2Strategy
			}
		} else {
			// on Kubernetes, the strategy might have been defined by the deprecated Spec.K8s.IngressStrategy
			if v1IngressStrategy != "" {
				// check for the default host
				if v1IngressStrategy == "default-host" {
					if v2Strategy != "single-host" {
						v1.Spec.K8s.IngressStrategy = v2Strategy
					}
				} else if v2Strategy != v1Strategy {
					// only change the strategy if the determined strategy would differ
					v1.Spec.K8s.IngressStrategy = v2Strategy
				}
			} else {
				if v2Strategy != v1Strategy {
					// only update if the v2Strategy doesn't correspond to the default determined from the v1
					v1.Spec.Server.ServerExposureStrategy = v2Strategy
				}
			}
		}
	} else {
		// The below table specifies how to convert the v2Strategy back to v1 taking into the account the original state of v1
		// from which v2 was converted before (which could also be just the default v1, if v2 was created on its own)
		//
		// v2Strategy | orig v1Strategy | orig v1ExposureType | resulting v1Strategy | resulting v1ExposureType
		// ----------------------------------------------------------------------------------------------------
		// single     | single          | native              | single               | orig
		// single     | single          | gateway             | single               | orig
		// single     | default         | NA                  | default              | orig
		// single     | multi           | NA                  | single               | orig
		// multi      | single          | native              | multi                | orig
		// multi      | single          | gateway             | multi                | orig
		// multi      | default         | NA                  | multi                | orig
		// multi      | multi           | NA                  | multi                | orig
		//
		// Notice that we don't ever want to update the singlehost exposure type. This is only used on Kubernetes and dictates how
		// we are going to expose the singlehost endpoints - either using ingresses (native) or using the gateway.
		// Because this distinction is not made in DWCO, which always uses the gateway, we just keep whatever the value was originally.
		//
		// The default-host is actually not supported in v2... but it is quite similar to single host in that everything is exposed
		// through the cluster hostname and when converting to v2, we convert it to single-host
		if v1Strategy != "default-host" || v2Strategy != "single-host" {
			v1.Spec.Server.ServerExposureStrategy = v2Strategy
		}
	}
}

func v2alpha1ToV1_GatewayImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.SingleHostGatewayImage = v2.Spec.Gateway.Image
}

func v2alpha1ToV1_GatewayConfigurerImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.SingleHostGatewayConfigSidecarImage = v2.Spec.Gateway.ConfigurerImage
}

func v2alpha1ToV1_GatewayTlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// see the comments in the v1 to v2alpha1 conversion method
	v1.Spec.Server.CheHostTLSSecret = v2.Spec.Gateway.TlsSecretName
}

func v2alpha1ToV1_GatewayConfigLabels(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.SingleHostGatewayConfigMapLabels = v2.Spec.Gateway.ConfigLabels
}

func v2alpha1ToV1_K8sIngressAnnotations(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	ingressClass := v2.Spec.K8s.IngressAnnotations["kubernetes.io/ingress.class"]
	if ingressClass == "" {
		ingressClass = defaultV2alpha1IngressClass
	}
	if v1.Spec.K8s.IngressClass != "" || ingressClass != defaultV1IngressClass {
		v1.Spec.K8s.IngressClass = ingressClass
	}
}
