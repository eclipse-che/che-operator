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
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	deployContext *DeployContext,
	name string,
	claimSize string,
	component string) (bool, error) {

	pvcSpec := getPVCSpec(deployContext, name, claimSize, component)

	actual := &corev1.PersistentVolumeClaim{}
	exists, err := GetNamespacedObject(deployContext, name, actual)
	if err != nil {
		return false, err
	} else if err == nil && exists {
		actual.Spec.Resources.Requests[corev1.ResourceName(corev1.ResourceStorage)] = resource.MustParse(claimSize)
		return Sync(deployContext, actual, pvcDiffOpts)
	}

	return Sync(deployContext, pvcSpec, pvcDiffOpts)
}

func getPVCSpec(
	deployContext *DeployContext,
	name string,
	claimSize string,
	component string) *corev1.PersistentVolumeClaim {

	labels := GetLabels(deployContext.CheCluster, component)
	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteOnce,
	}
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(claimSize),
		}}
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: accessModes,
		Resources:   resources,
	}
	if len(deployContext.CheCluster.Spec.Storage.PostgresPVCStorageClassName) > 1 {
		pvcSpec.StorageClassName = &deployContext.CheCluster.Spec.Storage.PostgresPVCStorageClassName
	}

	pvc := &corev1.PersistentVolumeClaim{
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

	return pvc
}
