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

package openvsx

import (
	"context"
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"github.com/eclipse-che/che-operator/pkg/common/reconciler"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXSecretReconciler struct {
	reconciler.Reconcilable
}

var logger = ctrl.Log.WithName("openvsx")

func NewOpenVSXSecretReconciler() *OpenVSXSecretReconciler {
	return &OpenVSXSecretReconciler{}
}

func (r *OpenVSXSecretReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	hasCustomCredentialsSecret, err := HasCustomCredentialsSecret(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("error checking OpenVSX Credentials secret: %w", err)
	}

	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		if !hasCustomCredentialsSecret {
			deleteResources(ctx)
		}
		return reconcile.Result{}, true, nil
	}

	err = r.syncSecret(ctx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Secret %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXSecretReconciler) Finalize(_ *chetypes.DeployContext) bool {
	return true
}

func (p *OpenVSXSecretReconciler) syncSecret(ctx *chetypes.DeployContext) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXCredentialsSecret,
			Namespace: ctx.CheCluster.Namespace,
			Labels:    deploy.GetLabels(constants.OpenVSXDatabaseComponentName),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"database-name":           []byte("openvsx"),
			"database-user":           []byte("openvsx"),
			"database-password":       []byte(utils.GeneratePassword(16)),
			"openvsx-publisher-name":  []byte("openvsx-publisher"),
			"openvsx-publisher-token": []byte(utils.GeneratePassword(32)),
			"openvsx-admin-name":      []byte("openvsx-admin"),
			"openvsx-admin-token":     []byte(utils.GeneratePassword(32)),
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, secret, ctx.ClusterAPI.Scheme); err != nil {
		return err
	}

	return ctx.ClusterAPI.ClientWrapper.CreateIfNotExists(context.TODO(), secret)
}

func HasCustomCredentialsSecret(ctx *chetypes.DeployContext) (bool, error) {
	credentialsSecretName := GetCredentialsSecretName(ctx)

	secret := &corev1.Secret{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: credentialsSecretName, Namespace: ctx.CheCluster.Namespace},
		secret,
	)
	if err != nil {
		return false, fmt.Errorf("failed to get secret: %w", err)
	}
	if exists {
		return !deploy.IsPartOfEclipseCheAndManagedByOperator(secret.Labels, constants.OpenVSXDatabaseComponentName), nil
	}

	return false, nil
}

func GetCredentialsSecretName(ctx *chetypes.DeployContext) string {
	return ptr.Deref(
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.CredentialsSecretName,
		constants.OpenVSXCredentialsSecret,
	)
}

func deleteResources(ctx *chetypes.DeployContext) {
	err := ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: constants.OpenVSXCredentialsSecret, Namespace: ctx.CheCluster.Namespace},
		&corev1.Secret{},
	)
	if err != nil {
		logger.Error(err, "Failed to delete OpenVSX credentials secret", "name", constants.OpenVSXCredentialsSecret)
	}
}
