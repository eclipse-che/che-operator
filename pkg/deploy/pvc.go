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

package deploy

import (
	chev2 "github.com/eclipse-che/che-operator/api/v2"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var pvcDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.PersistentVolumeClaim{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(corev1.PersistentVolumeClaimSpec{}, "VolumeName", "StorageClassName", "VolumeMode", "Selector", "DataSource"),
	cmpopts.IgnoreFields(corev1.ResourceRequirements{}, "Limits"),
	cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	}),
}

func SyncPVCToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	pvc *chev2.PVC,
	component string) (bool, error) {

	pvcSpec := getPVCSpec(deployContext, name, pvc, component)

	actual := &corev1.PersistentVolumeClaim{}
	exists, err := GetNamespacedObject(deployContext, name, actual)
	if err != nil {
		return false, err
	} else if exists {
		actual.Spec.Resources.Requests[corev1.ResourceName(corev1.ResourceStorage)] = resource.MustParse(pvc.ClaimSize)
		return Sync(deployContext, actual, pvcDiffOpts)
	}

	return Sync(deployContext, pvcSpec, pvcDiffOpts)
}

func getPVCSpec(
	deployContext *chetypes.DeployContext,
	name string,
	pvc *chev2.PVC,
	component string) *corev1.PersistentVolumeClaim {

	labels := GetLabels(component)
	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce,
	}
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(pvc.ClaimSize),
		}}
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: accessModes,
		Resources:   resources,
	}
	if pvc.StorageClass != "" {
		pvcSpec.StorageClassName = pointer.StringPtr(pvc.StorageClass)
	}

	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Spec: pvcSpec,
	}
}
