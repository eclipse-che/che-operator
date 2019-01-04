//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package operator

import (
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func newIngress(name string, serviceName string, port int) *v1beta1.Ingress {
	labels := cheLabels
	if name == "keycloak" {
		labels = keycloakLabels
	}
	tls := "false"
	if tlsSupport {
		tls = "true"
	}
	host := ""
	path := "/"
	if name == "keycloak" && strategy != "multi-host" {
		path = "/auth"
	}
	if strategy == "multi-host" {
		host = name + "-" + namespace + "." + ingressDomain
	} else if strategy == "single-host" {
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
			Name:     name,
			Namespace: namespace,
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

func createIngress(name string, serviceName string, port int) *v1beta1.Ingress {
	ingress := newIngress(name, serviceName, port)
	if err := sdk.Create(ingress); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+name+" ingress : %v", err)
		return nil
	}
	return ingress
}

func CreateIngressIfNotExists(name string, serviceName string, port int) *v1beta1.Ingress {
	ingress := newIngress(name, serviceName, port)
	err := sdk.Get(ingress)
	if err != nil {
		return createIngress(name, serviceName, port)
	}
	return ingress
}
