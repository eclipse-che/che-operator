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
	"strings"

	v1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/api/v2alpha1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
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

	v2.ObjectMeta = *v1.ObjectMeta.DeepCopy()
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
	v1ToV2alpha1_WorkspacePodNodeSelector(v1, v2)
	if err := v1ToV2alpha1_WorkspacePodTolerations(v1, v2); err != nil {
		return err
	}
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

	v1Obj.ObjectMeta = *v2.ObjectMeta.DeepCopy()
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
	v2alpha1ToV1_GatewayImage(v1Obj, v2)
	v2alpha1ToV1_GatewayConfigurerImage(v1Obj, v2)
	v2alpha1ToV1_GatewayTlsSecretName(v1Obj, v2)
	v2alpha1ToV1_GatewayConfigLabels(v1Obj, v2)
	v2alpha1ToV1_WorkspaceDomainEndpointsBaseDomain(v1Obj, v2)
	v2alpha1ToV1_WorkspaceDomainEndpointsTlsSecretName(v1Obj, v2)
	v2alpha1ToV1_WorkspacePodNodeSelector(v1Obj, v2)
	v2alpha1ToV1_WorkspacePodTolerations(v1Obj, v2)
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
		v2.Spec.Workspaces.DomainEndpoints.BaseDomain = v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey]
	} else {
		v2.Spec.Workspaces.DomainEndpoints.BaseDomain = v1.Spec.K8s.IngressDomain
	}
}

func v1ToV2alpha1_WorkspaceDomainEndpointsTlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// Che server always uses the default cluster certificate for subdomain workspace endpoints on OpenShift, and the K8s.TlsSecretName on K8s.
	// Because we're dealing with endpoints, let's try to use the secret on Kubernetes and nothing (e.g. the default cluster cert on OpenShift)
	// which is in line with the logic of the Che server.
	if !util.IsOpenShift {
		v2.Spec.Workspaces.DomainEndpoints.TlsSecretName = v1.Spec.K8s.TlsSecretName
	}
}

func v1ToV2alpha1_WorkspacePodNodeSelector(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	selector := v1.Spec.Server.WorkspacePodNodeSelector
	if len(selector) == 0 {
		prop := v1.Spec.Server.CustomCheProperties["CHE_WORKSPACE_POD_NODE__SELECTOR"]
		if prop != "" {
			selector = map[string]string{}
			kvs := strings.Split(prop, ",")
			for _, pair := range kvs {
				kv := strings.Split(pair, "=")
				if len(kv) == 2 {
					selector[kv[0]] = kv[1]
				}
			}
		}
	}
	v2.Spec.Workspaces.PodNodeSelector = selector
}

func v1ToV2alpha1_WorkspacePodTolerations(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) error {
	tolerations := v1.Spec.Server.WorkspacePodTolerations

	if len(tolerations) == 0 {
		prop := v1.Spec.Server.CustomCheProperties["CHE_WORKSPACE_POD_TOLERATIONS__JSON"]
		if prop != "" {
			tols := []corev1.Toleration{}
			if err := json.Unmarshal([]byte(prop), &tols); err != nil {
				return err
			}
			tolerations = tols
		}
	}

	v2.Spec.Workspaces.PodTolerations = tolerations
	return nil
}

func v1ToV2alpha1_GatewayEnabled(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// On Kubernetes, we can have single-host realized using ingresses (that use the same host but different paths).
	// This is actually not supported on DWCO where we always use the gateway for that. So here, we actually just
	// ignore the Spec.K8s.SingleHostExposureType, but we need to be aware of it when converting back.
	// Note that default-host is actually not supported on v2, but it is similar enough to default host that we
	// treat it as such for v2. The difference between default-host and single-host is that the default-host uses
	// the cluster domain itself as the base domain whereas single-host uses a configured domain. In v2 we always
	// need a domain configured.
	v2.Spec.Gateway.Enabled = pointer.BoolPtr(true)
}

func v1ToV2alpha1_GatewayImage(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v2.Spec.Gateway.Image = util.GetValue(v1.Spec.Server.SingleHostGatewayImage, deploy.DefaultSingleHostGatewayImage(v1))
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
		if len(v2.Spec.Workspaces.DomainEndpoints.BaseDomain) > 0 {
			v1.Spec.Server.CustomCheProperties[routeDomainSuffixPropertyKey] = v2.Spec.Workspaces.DomainEndpoints.BaseDomain
		}
	} else {
		v1.Spec.K8s.IngressDomain = v2.Spec.Workspaces.DomainEndpoints.BaseDomain
	}
}

func v2alpha1ToV1_WorkspaceDomainEndpointsTlsSecretName(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	// see the comments in the v1 to v2alpha1 conversion method
	if !util.IsOpenShift {
		v1.Spec.K8s.TlsSecretName = v2.Spec.Workspaces.DomainEndpoints.TlsSecretName
	}
}

func v2alpha1ToV1_WorkspacePodNodeSelector(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.WorkspacePodNodeSelector = v2.Spec.Workspaces.PodNodeSelector
}

func v2alpha1ToV1_WorkspacePodTolerations(v1 *v1.CheCluster, v2 *v2alpha1.CheCluster) {
	v1.Spec.Server.WorkspacePodTolerations = v2.Spec.Workspaces.PodTolerations
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
