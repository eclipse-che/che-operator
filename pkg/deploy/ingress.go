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
	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func NewIngress(cr *orgv1.CheCluster, name string, serviceName string, port int) *v1beta1.Ingress {
	tlsSupport := cr.Spec.Server.TlsSupport
	ingressStrategy := cr.Spec.K8s.IngressStrategy
	if len(ingressStrategy) < 1 {
		ingressStrategy = "multi-host"
	}
	ingressDomain := cr.Spec.K8s.IngressDomain
	ingressClass := util.GetValue(cr.Spec.K8s.IngressClass, DefaultIngressClass)
	labels := GetLabels(cr, name)

	tlsSecretName := cr.Spec.K8s.TlsSecretName
	tls := "false"
	if tlsSupport {
		tls = "true"
		// If TLS is turned on but the secret name is not set, try to use Che default value as k8s cluster defaults will not work.
		if tlsSecretName == "" {
			tlsSecretName = "che-tls"
		}
	}

	host := ""
	path := "/"
	if name == "keycloak" && ingressStrategy != "multi-host" {
		path = "/auth"
	}
	if ingressStrategy == "multi-host" {
		host = name + "-" + cr.Namespace + "." + ingressDomain
	} else if ingressStrategy == "single-host" {
		host = ingressDomain
	}
	tlsIngress := []v1beta1.IngressTLS{}
	if tlsSupport {
		tlsIngress = []v1beta1.IngressTLS{
			{
				Hosts: []string{
					ingressDomain,
				},
				SecretName: tlsSecretName,
			},
		}
	}

	return &v1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: v1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":                       ingressClass,
				"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
				"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
				"nginx.ingress.kubernetes.io/ssl-redirect":          tls,
			},
		},
		Spec: v1beta1.IngressSpec{
			TLS: tlsIngress,
			Rules: []v1beta1.IngressRule{
				{
					Host: host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{

							Paths: []v1beta1.HTTPIngressPath{
								{
									Backend: v1beta1.IngressBackend{
										ServiceName: serviceName,
										ServicePort: intstr.FromInt(port),
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
}
