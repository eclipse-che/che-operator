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

package authorization

import (
	"context"
	"fmt"
	"net/http"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	accessAllowedMsg       = "access allowed"
	accessDeniedErrorMsg   = "access denied"
	internalServerErrorMsg = "failed to evaluate authorization"
)

var logger = ctrl.Log.WithName("authorization")

type AuthorizationWebhook struct {
	client client.Client
}

func NewAuthorizationWebhook(client client.Client) *AuthorizationWebhook {
	return &AuthorizationWebhook{client: client}
}

func (w *AuthorizationWebhook) Handle(_ context.Context, req admission.Request) admission.Response {
	username := req.UserInfo.Username
	groups := req.UserInfo.Groups

	logger.Info("handle", "username", username, "groups", groups)

	cheCluster, err := deploy.FindCheClusterCRInNamespace(w.client, "")
	if err != nil {
		logger.Error(err, "Failed to find CheCluster Custom Resource")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf(internalServerErrorMsg))
	}
	if cheCluster == nil {
		logger.Info("CheCluster Custom Resource not found.")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf(internalServerErrorMsg))
	}

	if !IsAuthorized(username, groups, cheCluster.Spec.Networking.Auth.AdvancedAuthorization) {
		return admission.Errored(http.StatusForbidden, fmt.Errorf(accessDeniedErrorMsg))
	}

	return admission.Allowed(accessAllowedMsg)
}
