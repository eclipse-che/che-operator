//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package operator

import (
	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func pvc(name string, pvcClaimSize string, labels map[string]string) *corev1.PersistentVolumeClaim {

	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:     name,
			Namespace: namespace,
			Labels:    labels,

		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				// todo Make configurable
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(pvcClaimSize),
				},
			},
		},
	}

}
// CreatePVC creates a persistent volume claim with a given name, claim size, access mode and labels
func CreatePVC(name string, pvcClaimSize string, labels map[string]string) *corev1.PersistentVolumeClaim {
	pvc := pvc(name, pvcClaimSize, labels)
	if err := sdk.Create(pvc); err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create "+name+" PVC : %v", err)
		return nil
	}
	return pvc
}
