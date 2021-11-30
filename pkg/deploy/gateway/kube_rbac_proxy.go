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

package gateway

import (
	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/eclipse-che/che-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getGatewayKubeRbacProxyConfigSpec(instance *orgv1.CheCluster) corev1.ConfigMap {
	return corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "che-gateway-config-kube-rbac-proxy",
			Namespace: instance.Namespace,
			Labels:    deploy.GetLabels(instance, GatewayServiceName),
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

func getKubeRbacProxyContainerSpec(instance *orgv1.CheCluster) corev1.Container {
	authzImage := util.GetValue(instance.Spec.Auth.GatewayAuthorizationSidecarImage, deploy.DefaultGatewayAuthorizationSidecarImage(instance))
	return corev1.Container{
		Name:            "kube-rbac-proxy",
		Image:           authzImage,
		ImagePullPolicy: corev1.PullAlways,
		Args: []string{
			"--insecure-listen-address=0.0.0.0:8089",
			"--upstream=http://127.0.0.1:8090/ping",
			"--logtostderr=true",
			"--config-file=/etc/kube-rbac-proxy/authorization-config.yaml",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "kube-rbac-proxy-config",
				MountPath: "/etc/kube-rbac-proxy",
			},
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
