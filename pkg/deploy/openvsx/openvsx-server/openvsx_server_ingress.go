//
// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package openvsx_server

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	"github.com/eclipse-che/che-operator/pkg/common/infrastructure"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func (r *OpenVSXServerReconciler) syncIngress(ctx *chetypes.DeployContext) error {
	host := getHostName(ctx)

	labels := deploy.GetLabels(constants.OpenVSXServerComponentName)
	maps.Copy(labels, ctx.CheCluster.Spec.Networking.Labels)

	annotations := map[string]string{}
	if len(ctx.CheCluster.Spec.Networking.Annotations) > 0 {
		maps.Copy(annotations, ctx.CheCluster.Spec.Networking.Annotations)
	} else {
		maps.Copy(annotations, deploy.DefaultIngressAnnotations)
	}

	if infrastructure.IsOpenShift() {
		annotations["route.openshift.io/termination"] = "edge"
	}

	ingressClassName := ctx.CheCluster.Spec.Networking.IngressClassName
	if ingressClassName == "" {
		ingressClassName = annotations["kubernetes.io/ingress.class"]
		delete(annotations, "kubernetes.io/ingress.class")
	}

	paths := []networkingv1.HTTPIngressPath{
		{
			Path:     "/",
			PathType: ptr.To(networkingv1.PathTypePrefix),
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: constants.OpenVSXServerComponentName,
					Port: networkingv1.ServiceBackendPort{
						Number: constants.OpenVSXServerServicePort,
					},
				},
			},
		},
	}

	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.OpenVSXServerComponentName,
			Namespace:   ctx.CheCluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To(ingressClassName),
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{host},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: paths,
						},
					},
				},
			},
		},
	}

	if ctx.CheCluster.Spec.Networking.IngressClassName != "" {
		ingress.Spec.IngressClassName = ptr.To(ctx.CheCluster.Spec.Networking.IngressClassName)
	}

	labelKeys := slices.Collect(maps.Keys(labels))
	annotationKeys := slices.Collect(maps.Keys(annotations))

	return ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		ingress,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.Ingress(labelKeys, annotationKeys),
		},
	)
}

func (r *OpenVSXServerReconciler) syncOpenVSXURLStatus(ctx *chetypes.DeployContext) error {
	openVSXURL := "https://" + getHostName(ctx)

	if openVSXURL != ctx.CheCluster.Status.OpenVSXURL {
		ctx.CheCluster.Status.OpenVSXURL = openVSXURL

		if err := deploy.UpdateCheCRStatus(ctx, "status: OpenVSXURL", openVSXURL); err != nil {
			return fmt.Errorf("failed to update status for OpenVSXURL: %w", err)
		}
	}

	return nil
}

func getHostName(ctx *chetypes.DeployContext) string {
	return constants.OpenVSXServerHostPrefix + ctx.CheHost
}
