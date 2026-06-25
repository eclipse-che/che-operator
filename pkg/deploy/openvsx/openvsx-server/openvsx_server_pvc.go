// Copyright (c) 2019-2026 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//
//	Red Hat, Inc. - initial API and implementation

package openvsx_server

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *OpenVSXServerReconciler) syncPVC(ctx *chetypes.DeployContext) error {
	claimSize := constants.OpenVSXServerClaimSize
	
	if ctx.CheCluster.Spec.Components.OpenVSXRegistry.Server != nil &&
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.Server.Storage != nil &&
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.Server.Storage.ClaimSize != "" {

		claimSize = ctx.CheCluster.Spec.Components.OpenVSXRegistry.Server.Storage.ClaimSize
	}

	pvc := &corev1.PersistentVolumeClaim{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: ctx.CheCluster.Namespace,
		},
		pvc,
	)
	if err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	if exists {
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(claimSize)
		return ctx.ClusterAPI.ClientWrapper.Sync(context.TODO(), pvc)
	}

	pvc = &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXServerComponentName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXServerComponentName),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(claimSize),
				},
			},
		},
	}

	if err = controllerutil.SetControllerReference(ctx.CheCluster, pvc, ctx.ClusterAPI.Scheme); err != nil {
		return err
	}

	return ctx.ClusterAPI.ClientWrapper.CreateIfNotExists(context.TODO(), pvc)
}
