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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/diffs"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *OpenVSXServerReconciler) syncService(ctx *chetypes.DeployContext) error {
	labels := deploy.GetLabels(constants.OpenVSXServerComponentName)

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       constants.OpenVSXServerComponentName,
					Port:       constants.OpenVSXServerServicePort,
					TargetPort: intstr.FromInt32(constants.OpenVSXServerServicePort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: labels,
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, service, ctx.ClusterAPI.Scheme); err != nil {
		return err
	}

	return ctx.ClusterAPI.ClientWrapper.Sync(
		context.TODO(),
		service,
		&k8sclient.SyncOptions{
			DiffOpts: diffs.Service,
		})
}

func getServiceURL(ctx *chetypes.DeployContext) string {
	return fmt.Sprintf("http://%s.%s.svc:%d",
		constants.OpenVSXServerComponentName,
		ctx.CheCluster.Namespace,
		constants.OpenVSXServerServicePort,
	)
}
