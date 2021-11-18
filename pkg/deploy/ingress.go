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
package deploy

import (
	"reflect"
	"sort"
	"strconv"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/util"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ingressDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(networking.Ingress{}, "TypeMeta", "Status"),
	cmpopts.IgnoreFields(networking.HTTPIngressPath{}, "PathType"),
	cmp.Comparer(func(x, y metav1.ObjectMeta) bool {
		return reflect.DeepEqual(x.Labels, y.Labels) &&
			x.Annotations[CheEclipseOrgManagedAnnotationsDigest] == y.Annotations[CheEclipseOrgManagedAnnotationsDigest]
	}),
}

// SyncIngressToCluster creates ingress to expose service with the set settings
// host and path are evaluated if they are empty
func SyncIngressToCluster(
	deployContext *DeployContext,
	name string,
	host string,
	path string,
	serviceName string,
	servicePort int,
	ingressCustomSettings orgv1.IngressCustomSettings,
	component string) (endpointUrl string, done bool, err error) {

	ingressUrl, ingressSpec := GetIngressSpec(deployContext, name, host, path, serviceName, servicePort, ingressCustomSettings, component)
	sync, err := Sync(deployContext, ingressSpec, ingressDiffOpts)
	return ingressUrl, sync, err
}

// GetIngressSpec returns expected ingress config for given parameters
// host and path are evaluated if they are empty
func GetIngressSpec(
	deployContext *DeployContext,
	name string,
	host string,
	path string,
	serviceName string,
	servicePort int,
	ingressCustomSettings orgv1.IngressCustomSettings,
	component string) (ingressUrl string, i *networking.Ingress) {

	cheFlavor := DefaultCheFlavor(deployContext.CheCluster)
	tlsSupport := deployContext.CheCluster.Spec.Server.TlsSupport
	ingressStrategy := util.GetServerExposureStrategy(deployContext.CheCluster)
	ingressDomain := deployContext.CheCluster.Spec.K8s.IngressDomain
	tlsSecretName := deployContext.CheCluster.Spec.K8s.TlsSecretName
	ingressClass := util.GetValue(deployContext.CheCluster.Spec.K8s.IngressClass, DefaultIngressClass)
	labels := GetLabels(deployContext.CheCluster, component)
	MergeLabels(labels, ingressCustomSettings.Labels)
	pathType := networking.PathTypeImplementationSpecific

	if tlsSupport {
		// for server and dashboard ingresses
		if (component == cheFlavor || component == cheFlavor+"-dashboard") && deployContext.CheCluster.Spec.Server.CheHostTLSSecret != "" {
			tlsSecretName = deployContext.CheCluster.Spec.Server.CheHostTLSSecret
		}
	}

	if host == "" {
		if ingressStrategy == "multi-host" {
			host = component + "-" + deployContext.CheCluster.Namespace + "." + ingressDomain
		} else if ingressStrategy == "single-host" {
			host = ingressDomain
		}
	}

	var endpointPath, ingressPath string
	if path == "" {
		endpointPath, ingressPath = evaluatePath(component, ingressStrategy)
	} else {
		ingressPath = path
		endpointPath = path
	}

	annotations := map[string]string{
		"kubernetes.io/ingress.class":                       ingressClass,
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
		"nginx.ingress.kubernetes.io/ssl-redirect":          strconv.FormatBool(tlsSupport),
	}
	if ingressStrategy != "multi-host" && (component == DevfileRegistryName || component == PluginRegistryName) {
		annotations["nginx.ingress.kubernetes.io/rewrite-target"] = "/$1"
	}
	// Set bigger proxy buffer size to prevent 502 auth error.
	if component == IdentityProviderName {
		annotations["nginx.ingress.kubernetes.io/proxy-buffer-size"] = "16k"
	}
	for k, v := range ingressCustomSettings.Annotations {
		annotations[k] = v
	}

	// add 'che.eclipse.org/managed-annotations-digest' annotation
	// to store and compare annotations managed by operator only
	annotationsKeys := make([]string, 0, len(annotations))
	for k := range annotations {
		annotationsKeys = append(annotationsKeys, k)
	}
	if len(annotationsKeys) > 0 {
		sort.Strings(annotationsKeys)

		data := ""
		for _, k := range annotationsKeys {
			data += k + ":" + annotations[k] + ","
		}
		if util.IsTestMode() {
			annotations[CheEclipseOrgManagedAnnotationsDigest] = "0000"
		} else {
			annotations[CheEclipseOrgManagedAnnotationsDigest] = util.ComputeHash256([]byte(data))
		}
	}

	ingress := &networking.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: networking.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   deployContext.CheCluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networking.IngressSpec{
			Rules: []networking.IngressRule{
				{
					Host: host,
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: []networking.HTTPIngressPath{
								{
									Backend: networking.IngressBackend{
										Service: &networking.IngressServiceBackend{
											Name: serviceName,
											Port: networking.ServiceBackendPort{
												Number: int32(servicePort),
											},
										},
									},
									Path:     ingressPath,
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}

	if component == cheFlavor {
		// adds annotation, see details https://github.com/eclipse/che/issues/19434#issuecomment-810325262
		ingress.ObjectMeta.Annotations["nginx.org/websocket-services"] = serviceName
	}

	if tlsSupport {
		ingress.Spec.TLS = []networking.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: tlsSecretName,
			},
		}
	}

	return host + endpointPath, ingress
}

// evaluatePath evaluates ingress path (one which is used for rule)
// and endpoint path (one which client should use during endpoint accessing)
func evaluatePath(component, ingressStrategy string) (endpointPath, ingressPath string) {
	if ingressStrategy == "multi-host" {
		ingressPath = "/"
		endpointPath = "/"
		// Keycloak needs special rule in multihost. It's exposed on / which redirects to /auth
		// clients which does not support redirects needs /auth be explicitely set
		if component == IdentityProviderName {
			endpointPath = "/auth"
		}
	} else {
		switch component {
		case IdentityProviderName:
			endpointPath = "/auth"
			ingressPath = endpointPath + "/(.*)"
		case DevfileRegistryName:
			fallthrough
		case PluginRegistryName:
			endpointPath = "/" + component
			ingressPath = endpointPath + "/(.*)"
		default:
			ingressPath = "/"
			endpointPath = "/"
		}

	}
	return endpointPath, ingressPath
}
