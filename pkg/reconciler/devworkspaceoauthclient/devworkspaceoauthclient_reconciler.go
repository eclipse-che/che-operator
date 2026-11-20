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

package devworkspaceoauthclient

import (
	"fmt"

	dwconstants "github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/eclipse-che/che-operator/pkg/common/chelabels"
	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/eclipse-che/che-operator/pkg/common/utils"
	"github.com/eclipse-che/che-operator/pkg/provision/resources"
	"github.com/eclipse-che/che-operator/pkg/reconciler"
	oauthv1 "github.com/openshift/api/oauth/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	ClientSecretFile      = "client-secret"
	ClientSecretMountPath = "/var/run/oauth-client"

	DevWorkspaceOAuthClientSecret = "devworkspace-oauth-client-secret"
)

type WorkspacesOAuthClientReconciler struct {
	reconciler.Reconcilable
}

func NewReconciler() *WorkspacesOAuthClientReconciler {
	return &WorkspacesOAuthClientReconciler{}
}

func (c *WorkspacesOAuthClientReconciler) Reconcile(cheCtx *chetypes.CheContext) (reconcile.Result, bool, error) {
	if err := c.syncAll(cheCtx); err != nil {
		return reconcile.Result{},
			false,
			fmt.Errorf("failed to reconcile OAuthClient: %w", err)
	}

	return reconcile.Result{}, true, nil
}

func (c *WorkspacesOAuthClientReconciler) Finalize(cheCtx *chetypes.CheContext) error {
	return c.deleteAll(cheCtx)
}

func (c *WorkspacesOAuthClientReconciler) syncAll(cheCtx *chetypes.CheContext) error {
	clientSecret, err := c.syncSecret(cheCtx)
	if err != nil {
		return err
	}

	err = c.syncOAuthClient(cheCtx, clientSecret)
	if err != nil {
		return err
	}

	return nil
}

func (c *WorkspacesOAuthClientReconciler) syncSecret(cheCtx *chetypes.CheContext) (string, error) {
	secret := &corev1.Secret{}

	exists, err := resources.Secret.Get(cheCtx, DevWorkspaceOAuthClientSecret, cheCtx.CheCluster.Namespace, secret)
	if err != nil {
		return "", err
	}
	if exists {
		clientSecret := string(secret.Data[ClientSecretFile])
		return clientSecret, nil
	}

	clientSecret := utils.RandomString(32)

	secret = buildSecret(cheCtx, clientSecret)
	if err := resources.Secret.Create(cheCtx, secret); err != nil {
		return "", err
	}
	return clientSecret, nil
}

func (c *WorkspacesOAuthClientReconciler) syncOAuthClient(cheCtx *chetypes.CheContext, clientSecret string) error {
	oauthClient := buildOAuthClient(cheCtx, clientSecret)
	if err := resources.OAuthClient.Create(cheCtx, oauthClient); err != nil {
		return err
	}

	return nil
}

func (c *WorkspacesOAuthClientReconciler) deleteAll(cheCtx *chetypes.CheContext) error {
	return resources.OAuthClient.Delete(cheCtx, GetOAuthClientName())
}

func buildOAuthClient(cheCtx *chetypes.CheContext, clientSecret string) *oauthv1.OAuthClient {
	auth := cheCtx.CheCluster.Spec.Networking.Auth

	return &oauthv1.OAuthClient{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OAuthClient",
			APIVersion: oauthv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   GetOAuthClientName(),
			Labels: chelabels.GetStandardLabels(defaults.GetCheFlavor()),
		},
		Secret:                              clientSecret,
		RedirectURIs:                        []string{},
		GrantMethod:                         oauthv1.GrantHandlerPrompt,
		AccessTokenInactivityTimeoutSeconds: auth.OAuthAccessTokenInactivityTimeoutSeconds,
		AccessTokenMaxAgeSeconds:            auth.OAuthAccessTokenMaxAgeSeconds,
	}
}

func buildSecret(cheCtx *chetypes.CheContext, clientSecret string) *corev1.Secret {
	labels := chelabels.GetStandardLabels(constants.WorkspacesConfig)
	labels[dwconstants.DevWorkspaceWatchSecretLabel] = "true"
	labels[dwconstants.MountOnStartAttribute] = "true"

	annotations := map[string]string{
		dwconstants.DevWorkspaceMountAsAnnotation:   "file",
		dwconstants.DevWorkspaceMountPathAnnotation: ClientSecretMountPath,
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        DevWorkspaceOAuthClientSecret,
			Namespace:   cheCtx.CheCluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Data: map[string][]byte{
			ClientSecretFile: []byte(clientSecret),
		},
		Immutable: new(true),
	}
}

func GetOAuthClientName() string {
	return defaults.GetCheFlavor() + "-devworkspace"
}
