//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package openshiftoauth

import (
	"github.com/eclipse-che/che-operator/pkg/deploy"
	oauthv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Gets OpenShift user credentials secret from from the secret from:
// - openshift-config namespace
// - eclipse-che namespace
func GetOpenShiftOAuthUserCredentialsSecret(ctx *deploy.DeployContext) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	exists, err := deploy.Get(ctx, types.NamespacedName{Name: OpenShiftOAuthUserCredentialsSecret, Namespace: OcConfigNamespace}, secret)
	if err != nil {
		return nil, err
	} else if exists {
		return secret, nil
	}

	exists, err = deploy.GetNamespacedObject(ctx, OpenShiftOAuthUserCredentialsSecret, secret)
	if err != nil {
		return nil, err
	} else if exists {
		return secret, nil
	}

	return nil, nil
}

// Gets OpenShift OAuth.
func GetOpenshiftOAuth(ctx *deploy.DeployContext) (*oauthv1.OAuth, error) {
	oAuth := &oauthv1.OAuth{}
	if done, err := deploy.GetClusterObject(ctx, "cluster", oAuth); !done {
		return nil, err
	}
	return oAuth, nil
}
