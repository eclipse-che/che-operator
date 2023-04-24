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

	"k8s.io/utils/pointer"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/test"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
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
			x.Annotations[constants.CheEclipseOrgManagedAnnotationsDigest] == y.Annotations[constants.CheEclipseOrgManagedAnnotationsDigest]
	}),
}

var (
	DefaultIngressAnnotations = map[string]string{
		"kubernetes.io/ingress.class":                       "nginx",
		"nginx.ingress.kubernetes.io/proxy-read-timeout":    "3600",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout": "3600",
		"nginx.ingress.kubernetes.io/ssl-redirect":          "true",
	}
)

// SyncIngressToCluster creates ingress to expose service with the set settings
// host and path are evaluated if they are empty
func SyncIngressToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	path string,
	serviceName string,
	servicePort int,
	component string) (endpointUrl string, done bool, err error) {

	ingressUrl, ingressSpec := GetIngressSpec(deployContext, name, path, serviceName, servicePort, component)
	sync, err := Sync(deployContext, ingressSpec, ingressDiffOpts)
	return ingressUrl, sync, err
}

// GetIngressSpec returns expected ingress config for given parameters
// host and path are evaluated if they are empty
func GetIngressSpec(
	deployContext *chetypes.DeployContext,
	name string,
	path string,
	serviceName string,
	servicePort int,
	component string) (ingressUrl string, i *networking.Ingress) {

	ingressDomain := deployContext.CheCluster.Spec.Networking.Domain
	tlsSecretName := deployContext.CheCluster.Spec.Networking.TlsSecretName
	labels := GetLabels(component)
	for k, v := range deployContext.CheCluster.Spec.Networking.Labels {
		labels[k] = v
	}
	pathType := networking.PathTypeImplementationSpecific

	host := deployContext.CheCluster.Spec.Networking.Hostname
	if host == "" {
		host = ingressDomain
	}

	var endpointPath, ingressPath string
	if path == "" {
		endpointPath, ingressPath = evaluatePath(component)
	} else {
		ingressPath = path
		endpointPath = path
	}

	annotations := map[string]string{}
	if len(deployContext.CheCluster.Spec.Networking.Annotations) > 0 {
		for k, v := range deployContext.CheCluster.Spec.Networking.Annotations {
			annotations[k] = v
		}
	} else {
		for k, v := range DefaultIngressAnnotations {
			annotations[k] = v
		}

		// Set bigger proxy buffer size to prevent 502 auth error.
		annotations["nginx.ingress.kubernetes.io/proxy-buffer-size"] = "16k"
		annotations["nginx.org/websocket-services"] = serviceName
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
		if test.IsTestMode() {
			annotations[constants.CheEclipseOrgManagedAnnotationsDigest] = "0000"
		} else {
			annotations[constants.CheEclipseOrgManagedAnnotationsDigest] = utils.ComputeHash256([]byte(data))
		}
	}

	ingressClassName := deployContext.CheCluster.Spec.Networking.IngressClassName
	if ingressClassName == "" {
		ingressClassName = annotations["kubernetes.io/ingress.class"]
	}
	// annotations `kubernetes.io/ingress.class` can not be set when the class field is also set
	delete(annotations, "kubernetes.io/ingress.class")

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
			IngressClassName: pointer.String(ingressClassName),
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

	ingress.Spec.TLS = []networking.IngressTLS{
		{
			Hosts:      []string{host},
			SecretName: tlsSecretName,
		},
	}

	return host + endpointPath, ingress
}

// evaluatePath evaluates ingress path (one which is used for rule)
// and endpoint path (one which client should use during endpoint accessing)
func evaluatePath(component string) (endpointPath, ingressPath string) {
	switch component {
	case constants.DevfileRegistryName:
		fallthrough
	case constants.PluginRegistryName:
		endpointPath = "/" + component
		ingressPath = endpointPath + "/(.*)"
	default:
		ingressPath = "/"
		endpointPath = "/"
	}

	return endpointPath, ingressPath
}
