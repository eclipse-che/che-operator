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
	"github.com/sirupsen/logrus"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewPv(cr *orgv1.CheCluster, name string, pvClaimSize string, labels map[string]string) *corev1.PersistentVolume {
	hostPathType := new(corev1.HostPathType)
	*hostPathType = corev1.HostPathType(string(corev1.HostPathDirectoryOrCreate))
	accessModes := []corev1.PersistentVolumeAccessMode{
		// todo Make configurable
		corev1.ReadWriteOnce,
	}

	Requests := corev1.ResourceList{
		corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(pvClaimSize),
	}
	pvSpec := corev1.PersistentVolumeSpec{
		AccessModes: accessModes,
		Capacity:    Requests,
		PersistentVolumeSource: corev1.PersistentVolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: cr.Spec.Storage.PostgresPVCHostVolumePath,
				Type: hostPathType,
			},
		},
	}

	logrus.Info("Use postgres pv with storage class name: " + cr.Spec.Storage.PostgresPVCStorageClassName)
	logrus.Info("Use postgres pv with host volume path: " + cr.Spec.Storage.PostgresPVCHostVolumePath)

	if cr.Spec.Storage.PostgresPVCStorageClassName != "" {
		pvSpec.StorageClassName = cr.Spec.Storage.PostgresPVCStorageClassName
	}
	return &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: pvSpec,
	}

}

func NewPvc(cr *orgv1.CheCluster, name string, pvcClaimSize string, labels map[string]string) *corev1.PersistentVolumeClaim {

	accessModes := []corev1.PersistentVolumeAccessMode{
		// todo Make configurable
		corev1.ReadWriteOnce,
	}
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(pvcClaimSize),
		},
	}

	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: accessModes,
		Resources:   resources,
	}
	if cr.Spec.Storage.PostgresPVCHostVolumePath != "" {
		pvcSpec.VolumeName = name
	}
	if cr.Spec.Storage.PostgresPVCStorageClassName != "" {
		pvcSpec.StorageClassName = &cr.Spec.Storage.PostgresPVCStorageClassName
	}
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: pvcSpec,
	}

}
