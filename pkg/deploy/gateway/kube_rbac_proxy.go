//
// Copyright (c) 2019-2023 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package gateway

import (
	"strconv"

	"k8s.io/apimachinery/pkg/util/intstr"

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getGatewayKubeRbacProxyConfigSpec(instance *chev2.CheCluster) corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config-kube-rbac-proxy",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(GatewayServiceName),
		},
		Data: map[string]string{
			"authorization-config.yaml": `
authorization:
  rewrites:
    byQueryParameter:
      name: "namespace"
  resourceAttributes:
    apiVersion: v1
    apiGroup: workspace.devfile.io
    resource: devworkspaces
    namespace: "{{ .Value }}"`,
		},
	}
}

func getKubeRbacProxyContainerSpec(instance *chev2.CheCluster) corev1.Container {
	logLevel := constants.DefaultKubeRbacProxyLogLevel
	if instance.Spec.Networking.Auth.Gateway.KubeRbacProxy != nil && instance.Spec.Networking.Auth.Gateway.KubeRbacProxy.LogLevel != nil {
		logLevel = *instance.Spec.Networking.Auth.Gateway.KubeRbacProxy.LogLevel
	}

	return corev1.Container{
		Name:            "kube-rbac-proxy",
		Image:           defaults.GetGatewayAuthorizationSidecarImage(instance),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"--insecure-listen-address=0.0.0.0:8089",
			"--upstream=http://127.0.0.1:8090/ping",
			"--logtostderr=true",
			"--config-file=/etc/kube-rbac-proxy/authorization-config.yaml",
			"--v=" + strconv.FormatInt(int64(logLevel), 10),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "kube-rbac-proxy-config",
				MountPath: "/etc/kube-rbac-proxy",
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.5"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
				corev1.ResourceCPU:    resource.MustParse("0.1"),
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/ping",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(8090),
					},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 5,
			TimeoutSeconds:      5,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/ping",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(8090),
					},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      5,
			PeriodSeconds:       5,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		},
	}
}

func getKubeRbacProxyConfigVolume() corev1.Volume {
	return corev1.Volume{
		Name: "kube-rbac-proxy-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "che-gateway-config-kube-rbac-proxy",
				},
			},
		},
	}
}
