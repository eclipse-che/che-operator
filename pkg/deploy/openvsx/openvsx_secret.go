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
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type OpenVSXSecretReconciler struct {
	reconciler.Reconcilable
}

func NewOpenVSXSecretReconciler() *OpenVSXSecretReconciler {
	return &OpenVSXSecretReconciler{}
}

func (r *OpenVSXSecretReconciler) Reconcile(ctx *chetypes.DeployContext) (reconcile.Result, bool, error) {
	hasCustomCredentialsSecret, err := HasCustomCredentialsSecret(ctx)
	if err != nil {
		return reconcile.Result{}, false, err
	}

	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		if !hasCustomCredentialsSecret {
			_ = ctx.ClusterAPI.ClientWrapper.DeleteByKeyIgnoreNotFound(
				context.TODO(),
				types.NamespacedName{
					Name:      constants.OpenVSXCredentialsSecret,
					Namespace: ctx.CheCluster.Namespace,
				},
				&corev1.Secret{},
			)
		}

		return reconcile.Result{}, true, nil
	}

	if !ctx.CheCluster.IsInternalOpenVSXRegistryEnabled() {
		_, _ = deploy.DeleteNamespacedObject(ctx, OpenVSXIngressName, &networking.Ingress{})

		if ctx.CheCluster.Status.OpenVSXURL != "" {
			ctx.CheCluster.Status.OpenVSXURL = ""
			err := deploy.UpdateCheCRStatus(ctx, "OpenVSXURL", "")
			return reconcile.Result{}, err == nil, err
		}

		return reconcile.Result{}, true, nil
	}

	return reconcile.Result{}, true, nil
}

func (r *OpenVSXSecretReconciler) Finalize(ctx *chetypes.DeployContext) bool {
	return true
}

func (p *OpenVSXSecretReconciler) syncSecret(ctx *chetypes.DeployContext) (bool, error) {
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
			"database-name":     []byte("openvsx"),
			"database-user":     []byte("openvsx"),
			"database-password": []byte(utils.GeneratePassword(16)),
		},
	}

	if err := controllerutil.SetControllerReference(ctx.CheCluster, secret, ctx.ClusterAPI.Scheme); err != nil {
		return false, err
	}

	err := ctx.ClusterAPI.ClientWrapper.CreateIfNotExists(context.TODO(), secret)
	return err == nil, err
}

func HasCustomCredentialsSecret(ctx *chetypes.DeployContext) (bool, error) {
	credentialsSecretName := GetCredentialsSecretName(ctx)

	secret := &corev1.Secret{}
	exists, err := ctx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		context.TODO(),
		types.NamespacedName{Name: credentialsSecretName, Namespace: ctx.CheCluster.Namespace},
		&corev1.Secret{},
	)
	if err != nil {
		return false, fmt.Errorf("error getting credentials secret %s: %w", credentialsSecretName, err)
	}
	if exists {
		return !deploy.IsPartOfEclipseCheResourceAndManagedByOperator(secret.Labels), nil
	}

	return false, nil
}

func GetCredentialsSecretName(ctx *chetypes.DeployContext) string {
	return ptr.Deref(
		ctx.CheCluster.Spec.Components.OpenVSXRegistry.CredentialsSecretName,
		constants.OpenVSXCredentialsSecret,
	)
}
