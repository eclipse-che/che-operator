//
// Copyright (c) 2012-2020 Red Hat, Inc.
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
	"context"
	"fmt"

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PVCProvisioningStatus struct {
	ProvisioningStatus
}

var pvcDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.PersistentVolumeClaim{}, "TypeMeta", "ObjectMeta", "Status"),
	cmpopts.IgnoreFields(corev1.PersistentVolumeClaimSpec{}, "VolumeName", "StorageClassName", "VolumeMode"),
	cmp.Comparer(func(x, y resource.Quantity) bool {
		return x.Cmp(y) == 0
	}),
}

func SyncPVCToCluster(
	checluster *orgv1.CheCluster,
	name string,
	claimSize string,
	labels map[string]string,
	clusterAPI ClusterAPI) PVCProvisioningStatus {

	specPVC, err := getSpecPVC(checluster, name, claimSize, labels, clusterAPI.Scheme)
	if err != nil {
		return PVCProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	clusterPVC, err := getClusterPVC(specPVC.Name, specPVC.Namespace, clusterAPI.Client)
	if err != nil {
		return PVCProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Err: err},
		}
	}

	if clusterPVC == nil {
		logrus.Infof("Creating a new object: %s, name %s", specPVC.Kind, specPVC.Name)
		err := clusterAPI.Client.Create(context.TODO(), specPVC)
		return PVCProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	diff := cmp.Diff(clusterPVC, specPVC, pvcDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterPVC.Kind, clusterPVC.Name)
		fmt.Printf("Difference:\n%s", diff)
		clusterPVC.Spec = specPVC.Spec
		err := clusterAPI.Client.Update(context.TODO(), clusterPVC)
		return PVCProvisioningStatus{
			ProvisioningStatus: ProvisioningStatus{Requeue: true, Err: err},
		}
	}

	// Don't check Status.Phase == "Bound"
	// Sometimes PVC can be bound only when the first consumer is created

	return PVCProvisioningStatus{
		ProvisioningStatus: ProvisioningStatus{Continue: true},
	}
}

func getSpecPVC(
	checluster *orgv1.CheCluster,
	name string,
	claimSize string,
	labels map[string]string,
	scheme *runtime.Scheme) (*corev1.PersistentVolumeClaim, error) {

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
	if len(checluster.Spec.Storage.PostgresPVCStorageClassName) > 1 {
		pvcSpec = corev1.PersistentVolumeClaimSpec{
			AccessModes:      accessModes,
			StorageClassName: &checluster.Spec.Storage.PostgresPVCStorageClassName,
			Resources:        resources,
		}
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: checluster.Namespace,
			Labels:    labels,
		},
		Spec: pvcSpec,
	}

	err := controllerutil.SetControllerReference(checluster, pvc, scheme)
	if err != nil {
		return nil, err
	}

	return pvc, nil
}

func getClusterPVC(name string, namespace string, client runtimeClient.Client) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := client.Get(context.TODO(), namespacedName, pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return pvc, nil
}
