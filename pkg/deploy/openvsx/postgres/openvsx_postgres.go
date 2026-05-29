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

package postgres

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	pvcName      = "openvsx-postgres-data"
	postgresPort = int32(5432)
)

type OpenVSXPostgresReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXPostgresReconciler() *OpenVSXPostgresReconciler {
	return &OpenVSXPostgresReconciler{}
}

func (p *OpenVSXPostgresReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsOpenVSXOperandEnabled() {
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXPostgresName, &appsv1.Deployment{})
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXPostgresName, &corev1.Service{})
		_, _ = deploy.DeleteNamespacedObject(ctx, pvcName, &corev1.PersistentVolumeClaim{})
		_, _ = deploy.DeleteNamespacedObject(ctx, constants.OpenVSXPostgresCredentialsSecret, &corev1.Secret{})
		return reconcile.Result{}, true, nil
	}

	done, err := p.syncSecret(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncService(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncPVC(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	done, err = p.syncDeployment(ctx)
	if !done {
		return reconcile.Result{}, false, err
	}

	return reconcile.Result{}, true, nil
}

func (p *OpenVSXPostgresReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (p *OpenVSXPostgresReconciler) syncSecret(ctx *chetypes.DeployContext) (bool, error) {
	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      constants.OpenVSXPostgresCredentialsSecret,
		Namespace: ctx.CheCluster.Namespace,
	}, secret)

	if err == nil {
		return true, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	data := map[string][]byte{
		"user":     []byte("openvsx"),
		"password": []byte(utils.GeneratePassword(16)),
		"database": []byte("openvsx"),
	}

	return deploy.SyncSecretToCluster(ctx, constants.OpenVSXPostgresCredentialsSecret, ctx.CheCluster.Namespace, data)
}

func (p *OpenVSXPostgresReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.OpenVSXPostgresName,
		[]string{"tcp-postgresql"},
		[]int32{postgresPort},
		constants.OpenVSXPostgresName)
}

func (p *OpenVSXPostgresReconciler) syncPVC(ctx *chetypes.DeployContext) (bool, error) {
	claimSize := constants.DefaultOpenVSXPostgresClaimSize
	if ctx.CheCluster.Spec.Components.OpenVSX.Postgres != nil &&
		ctx.CheCluster.Spec.Components.OpenVSX.Postgres.ClaimSize != "" {
		claimSize = ctx.CheCluster.Spec.Components.OpenVSX.Postgres.ClaimSize
	}

	pvc := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXPostgresName),
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

	pvcDiffOpts := cmp.Options{
		cmpopts.IgnoreFields(corev1.PersistentVolumeClaim{}, "TypeMeta", "ObjectMeta", "Status"),
		cmpopts.IgnoreFields(corev1.PersistentVolumeClaimSpec{}, "VolumeName", "StorageClassName", "VolumeMode"),
	}
	return deploy.Sync(ctx, pvc, pvcDiffOpts)
}

func (p *OpenVSXPostgresReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := p.getDeploymentSpec(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}
