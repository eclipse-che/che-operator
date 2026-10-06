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

func (r *OpenVSXSecretReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	hasCustomCredentialsSecret, err := HasCustomCredentialsSecret(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("error checking OpenVSX Credentials secret: %w", err)
	}

	if !cheCtx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		if !hasCustomCredentialsSecret {
			deleteResources(cheCtx)
		}
		return reconcile.Result{}, true, nil
	}

	err = r.syncSecret(cheCtx)
	if err != nil {
		return reconcile.Result{}, false, fmt.Errorf("failed to sync Secret %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXSecretReconciler) Finalize(_ *chetypes.CheContext) bool {
	return true
}

func (p *OpenVSXSecretReconciler) syncSecret(cheCtx *chetypes.CheContext) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenVSXCredentialsSecret,
			Namespace: cheCtx.CheCluster.Namespace,
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

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, secret, cheCtx.ClusterAPI.Scheme); err != nil {
		return err
	}

	return cheCtx.ClusterAPI.ClientWrapper.CreateIfNotExists(context.TODO(), secret)
}

func HasCustomCredentialsSecret(cheCtx *chetypes.CheContext) (bool, error) {
	credentialsSecretName := GetCredentialsSecretName(cheCtx)

	secret := &corev1.Secret{}
	exists, err := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: credentialsSecretName, Namespace: cheCtx.CheCluster.Namespace},
		secret,
	)
	if err != nil {
		return false, fmt.Errorf("failed to get secret: %w", err)
	}
	if exists {
		return !deploy.IsOperatorManagedComponent(secret.Labels, constants.OpenVSXDatabaseComponentName), nil
	}

	return false, nil
}

func GetCredentialsSecretName(cheCtx *chetypes.CheContext) string {
	return ptr.Deref(
		cheCtx.CheCluster.Spec.Components.OpenVSXRegistry.CredentialsSecretName,
		constants.OpenVSXCredentialsSecret,
	)
}

func deleteResources(cheCtx *chetypes.CheContext) {
	err := cheCtx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: constants.OpenVSXCredentialsSecret, Namespace: cheCtx.CheCluster.Namespace},
		&corev1.Secret{},
	)
	if err != nil {
		logger.Error(err, "Failed to delete OpenVSX credentials secret", "name", constants.OpenVSXCredentialsSecret)
	}
}
