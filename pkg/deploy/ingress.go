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
	"context"
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/eclipse/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var ingressDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(v1beta1.Ingress{}, "TypeMeta", "ObjectMeta", "Status"),
}

func SyncIngressToCluster(
	checluster *orgv1.CheCluster,
	name string,
	host string,
	serviceName string,
	servicePort int,
	clusterAPI ClusterAPI) (*v1beta1.Ingress, error) {

	specIngress, err := getSpecIngress(checluster, name, host, serviceName, servicePort, clusterAPI)
	if err != nil {
		return nil, err
	}

	clusterIngress, err := getClusterIngress(specIngress.Name, specIngress.Namespace, clusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterIngress == nil {
		logrus.Infof("Creating a new object: %s, name %s", specIngress.Kind, specIngress.Name)
		err := clusterAPI.Client.Create(context.TODO(), specIngress)
		return nil, err
	}

	diff := cmp.Diff(clusterIngress, specIngress, ingressDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterIngress.Kind, clusterIngress.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := clusterAPI.Client.Delete(context.TODO(), clusterIngress)
		if err != nil {
			return nil, err
		}

		err = clusterAPI.Client.Create(context.TODO(), specIngress)
		return nil, err
	}

	return clusterIngress, nil
}

func DeleteIngressIfExists(name string, namespace string, clusterAPI ClusterAPI) error {
	ingress, err := getClusterIngress(name, namespace, clusterAPI.Client)
	if err != nil {
		return err
	}

	if ingress != nil {
		err = clusterAPI.Client.Delete(context.TODO(), ingress)
		if err != nil {
			return err
		}
	}

	return nil
}

func getClusterIngress(name string, namespace string, client runtimeClient.Client) (*v1beta1.Ingress, error) {
	ingress := &v1beta1.Ingress{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, ingress)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ingress, nil
}

func getSpecIngress(
	checluster *orgv1.CheCluster,
	name string,
	host string,
	serviceName string,
	servicePort int,
	clusterAPI ClusterAPI) (*v1beta1.Ingress, error) {

	tlsSupport := checluster.Spec.Server.TlsSupport
	ingressStrategy := util.GetServerExposureStrategy(checluster, DefaultServerExposureStrategy)
	ingressDomain := checluster.Spec.K8s.IngressDomain
	ingressClass := util.GetValue(checluster.Spec.K8s.IngressClass, DefaultIngressClass)
	labels := GetLabels(checluster, name)

	if host == "" {
		if ingressStrategy == "multi-host" {
			host = name + "-" + checluster.Namespace + "." + ingressDomain
		} else if ingressStrategy == "single-host" {
			host = ingressDomain
		}
	}

	tls := "false"
	tlsSecretName := util.GetValue(checluster.Spec.K8s.TlsSecretName, "che-tls")
	if tlsSupport {
		tls = "true"
		if name == DefaultCheFlavor(checluster) && checluster.Spec.Server.CheHostTLSSecret != "" {
			tlsSecretName = checluster.Spec.Server.CheHostTLSSecret
		}
	}

	path := "/"
	if ingressStrategy != "multi-host" {
		switch name {
		case "keycloak":
			path = "/auth"
		case DevfileRegistry:
			path = "/" + DevfileRegistry + "/(.*)"
		case PluginRegistry:
			path = "/" + PluginRegistry + "/(.*)"
		}
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class":                       ingressClass,
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
		"nginx.ingress.kubernetes.io/ssl-redirect":          tls,
	}
	if ingressStrategy != "multi-host" && (name == DevfileRegistry || name == PluginRegistry) {
		annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$1"
	}

	ingress := &v1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: v1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   checluster.Namespace,
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

	err := controllerutil.SetControllerReference(checluster, ingress, clusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return ingress, nil
}
