//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	"reflect"
	"strconv"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var ingressDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(v1beta1.Ingress{}, "TypeMeta", "Status"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels)
	}),
}

func SyncIngressToCluster(
	deployContext *DeployContext,
	name string,
	host string,
	serviceName string,
	servicePort int,
	ingressCustomSettings orgv1.IngressCustomSettings,
	component string) (bool, error) {

	ingressSpec := GetIngressSpec(deployContext, name, host, serviceName, servicePort, ingressCustomSettings, component)
	return Sync(deployContext, ingressSpec, ingressDiffOpts)
}

// GetIngressSpec returns expected ingress config for given parameters
func GetIngressSpec(
	deployContext *DeployContext,
	name string,
	host string,
	serviceName string,
	servicePort int,
	ingressCustomSettings orgv1.IngressCustomSettings,
	component string) *v1beta1.Ingress {

	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
	ingressStrategy := util.GetServerExposureStrategy(deployContext.CheCluster, DefaultServerExposureStrategy)
	ingressDomain := deployContext.CheCluster.Spec.K8s.IngressDomain
	ingressClass := util.GetValue(deployContext.CheCluster.Spec.K8s.IngressClass, DefaultIngressClass)
	labels := GetLabels(deployContext.CheCluster, component)
	MergeLabels(labels, ingressCustomSettings.Labels)

	if host == "" {
		if ingressStrategy == "multi-host" {
			host = name + "-" + deployContext.CheCluster.Namespace + "." + ingressDomain
		} else if ingressStrategy == "single-host" {
			host = ingressDomain
		}
	}

	tlsSecretName := util.GetValue(deployContext.CheCluster.Spec.K8s.TlsSecretName, "")
	if tlsSupport {
		if name == DefaultCheFlavor(deployContext.CheCluster) && deployContext.CheCluster.Spec.Server.CheHostTLSSecret != "" {
			tlsSecretName = deployContext.CheCluster.Spec.Server.CheHostTLSSecret
		}
	}

	path := "/"
	if ingressStrategy != "multi-host" {
		switch name {
		case IdentityProviderName:
			path = "/auth"
		case DevfileRegistryName:
			path = "/" + DevfileRegistryName + "/(.*)"
		case PluginRegistryName:
			path = "/" + PluginRegistryName + "/(.*)"
		}
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class":                       ingressClass,
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
		"nginx.ingress.kubernetes.io/ssl-redirect":          strconv.FormatBool(tlsSupport),
	}
	if ingressStrategy != "multi-host" && (name == DevfileRegistryName || name == PluginRegistryName) {
		annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$1"
	}

	ingress := &v1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: v1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   deployContext.CheCluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: serviceName,
										ServicePort: intstr.FromInt(servicePort),
									},
									Path: path,
								},
							},
						},
					},
				},
			},
		},
	}

	if tlsSupport {
		ingress.Spec.TLS = []v1beta1.IngressTLS{
			{
				Hosts: []string{
					ingressDomain,
				},
				SecretName: tlsSecretName,
			},
		}
	}

	return ingress
}
