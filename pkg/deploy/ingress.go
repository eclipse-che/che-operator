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
	"reflect"
	"strconv"

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
	additionalLabels string) (*v1beta1.Ingress, error) {

	specIngress, err := GetSpecIngress(deployContext, name, host, serviceName, servicePort, additionalLabels)
	if err != nil {
		return nil, err
	}

	clusterIngress, err := GetClusterIngress(specIngress.Name, specIngress.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return nil, err
	}

	if clusterIngress == nil {
		logrus.Infof("Creating a new object: %s, name %s", specIngress.Kind, specIngress.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specIngress)
		return nil, err
	}

	diff := cmp.Diff(clusterIngress, specIngress, ingressDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterIngress.Kind, clusterIngress.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := deployContext.ClusterAPI.Client.Delete(context.TODO(), clusterIngress)
		if err != nil {
			return nil, err
		}

		err = deployContext.ClusterAPI.Client.Create(context.TODO(), specIngress)
		return nil, err
	}

	return clusterIngress, nil
}

// DeleteIngressIfExists removes specified ingress if any
func DeleteIngressIfExists(name string, deployContext *DeployContext) error {
	ingress, err := GetClusterIngress(name, deployContext.CheCluster.Namespace, deployContext.ClusterAPI.Client)
	if err != nil {
		return err
	}

	if ingress != nil {
		err = deployContext.ClusterAPI.Client.Delete(context.TODO(), ingress)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetClusterIngress returns actual ingress config by provided name and namespace
func GetClusterIngress(name string, namespace string, client runtimeClient.Client) (*v1beta1.Ingress, error) {
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

// GetSpecIngress returns expected ingress config for given parameters
func GetSpecIngress(
	deployContext *DeployContext,
	name string,
	host string,
	serviceName string,
	servicePort int,
	additionalLabels string) (*v1beta1.Ingress, error) {

	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
	ingressStrategy := util.GetServerExposureStrategy(deployContext.CheCluster, DefaultServerExposureStrategy)
	ingressDomain := deployContext.CheCluster.Spec.K8s.IngressDomain
	ingressClass := util.GetValue(deployContext.CheCluster.Spec.K8s.IngressClass, DefaultIngressClass)
	labels := GetLabels(deployContext.CheCluster, name)
	MergeLabels(labels, additionalLabels)

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

	err := controllerutil.SetControllerReference(deployContext.CheCluster, ingress, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return ingress, nil
}
