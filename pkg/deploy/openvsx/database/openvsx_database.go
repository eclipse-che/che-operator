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

package database

import (
	"context"

	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	pvcName      = "openvsx-database-data"
	postgresPort = int32(5432)
)

type OpenVSXDatabaseReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXDatabaseReconciler() *OpenVSXDatabaseReconciler {
	return &OpenVSXDatabaseReconciler{}
}

func (p *OpenVSXDatabaseReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		ns := ctx.CheCluster.Namespace
		cw := ctx.ClusterAPI.ClientWrapper
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: ns}, &appsv1.Deployment{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseName, Namespace: ns}, &corev1.Service{})
		_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: ns}, &corev1.PersistentVolumeClaim{})
		if !isUserProvidedSecret(ctx) {
			_ = cw.DeleteByKeyIgnoreNotFound(context.TODO(), types.NamespacedName{Name: constants.OpenVSXDatabaseCredentialsSecret, Namespace: ns}, &corev1.Secret{})
		}
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

func (p *OpenVSXDatabaseReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

var requiredSecretKeys = []string{
	"db-user", "db-password", "db-name",
	"publisher-name", "publisher-token",
	"admin-name", "admin-token",
}

func (p *OpenVSXDatabaseReconciler) syncSecret(ctx *chetypes.DeployContext) (bool, error) {
	secretName := GetCredentialsSecretName(ctx)

	if isUserProvidedSecret(ctx) {
		secret := &corev1.Secret{}
		err := ctx.ClusterAPI.NonCachingClient.Get(context.TODO(), types.NamespacedName{
			Name:      secretName,
			Namespace: ctx.CheCluster.Namespace,
		}, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, fmt.Errorf("credentials secret '%s' not found", secretName)
			}
			return false, err
		}
		return validateSecretKeys(secret)
	}

	secret := &corev1.Secret{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      secretName,
		Namespace: ctx.CheCluster.Namespace,
	}, secret)

	if err == nil {
		return true, nil
	}

	if !errors.IsNotFound(err) {
		return false, err
	}

	secret = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(defaults.GetCheFlavor()),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"db-user":         []byte("openvsx"),
			"db-password":     []byte(utils.GeneratePassword(16)),
			"db-name":         []byte("openvsx"),
			"publisher-name":  []byte("eclipse-che"),
			"publisher-token": []byte(utils.GeneratePassword(32)),
			"admin-name":      []byte("openvsx-admin"),
			"admin-token":     []byte(utils.GeneratePassword(32)),
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, secret, ctx.ClusterAPI.Scheme); err != nil {
		return false, err
	}
	err = ctx.ClusterAPI.ClientWrapper.Sync(context.TODO(), secret)
	return err == nil, err
}

func validateSecretKeys(secret *corev1.Secret) (bool, error) {
	var missing []string
	for _, key := range requiredSecretKeys {
		if _, ok := secret.Data[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return false, fmt.Errorf("credentials secret '%s' is missing required keys: %v", secret.Name, missing)
	}
	return true, nil
}

func isUserProvidedSecret(ctx *chetypes.DeployContext) bool {
	return ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database != nil &&
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database.CredentialsSecretName != ""
}

func GetCredentialsSecretName(ctx *chetypes.DeployContext) string {
	if isUserProvidedSecret(ctx) {
		return ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database.CredentialsSecretName
	}
	return constants.OpenVSXDatabaseCredentialsSecret
}

func (p *OpenVSXDatabaseReconciler) syncService(ctx *chetypes.DeployContext) (bool, error) {
	return deploy.SyncServiceToCluster(
		ctx,
		constants.OpenVSXDatabaseName,
		[]string{"tcp-postgresql"},
		[]int32{postgresPort},
		constants.OpenVSXDatabaseName)
}

func (p *OpenVSXDatabaseReconciler) syncPVC(ctx *chetypes.DeployContext) (bool, error) {
	claimSize := constants.OpenVSXDatabaseClaimSize
	if ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database != nil &&
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database.Storage != nil &&
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database.Storage.ClaimSize != "" {
		claimSize = ctx.CheCluster.Spec.Components.OpenVSXRegistry.Database.Storage.ClaimSize
	}

	desiredSize := resource.MustParse(claimSize)

	existing := &corev1.PersistentVolumeClaim{}
	err := ctx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{
		Name:      pvcName,
		Namespace: ctx.CheCluster.Namespace,
	}, existing)

	if errors.IsNotFound(err) {
		pvc := &corev1.PersistentVolumeClaim{
			TypeMeta: metav1.TypeMeta{
				Kind:       "PersistentVolumeClaim",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: ctx.CheCluster.Namespace,
				Labels:    deploy.GetLabels(constants.OpenVSXDatabaseName),
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: desiredSize,
					},
				},
			},
		}
		return deploy.Sync(ctx, pvc, deploy.DefaultDeploymentDiffOpts)
	}
	if err != nil {
		return false, err
	}

	currentSize := existing.Spec.Resources.Requests[corev1.ResourceStorage]
	if desiredSize.Cmp(currentSize) > 0 {
		existing.Spec.Resources.Requests[corev1.ResourceStorage] = desiredSize
		return false, ctx.ClusterAPI.Client.Update(context.TODO(), existing)
	}

	return true, nil
}

func (p *OpenVSXDatabaseReconciler) syncDeployment(ctx *chetypes.DeployContext) (bool, error) {
	spec, err := p.getDeploymentSpec(ctx)
	if err != nil {
		return false, err
	}

	return deploy.SyncDeploymentSpecToCluster(ctx, spec, deploy.DefaultDeploymentDiffOpts)
}
