//
// Copyright (c) 2019-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package backup_servers

import (
	"context"
	"fmt"
	"strconv"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getResticRepoPassword checks if the password for restic repository is specified and retrieves it.
// It doesn't check the password correctness.
// Returns:
//  - password or empty string if password is not set
//  - done status
//  - error if any
func getResticRepoPassword(client client.Client, namespace string, repoPasswordSecretRef string) (string, bool, error) {
	if repoPasswordSecretRef == "" {
		return "", true, fmt.Errorf("restic repository password secret should be specified in %s field", chev1.RESTIC_REPO_PASSWORD_SECRET_KEY)
	}
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: repoPasswordSecretRef}
	err := client.Get(context.TODO(), namespacedName, secret)
	if err == nil {
		password, exist := secret.Data[chev1.RESTIC_REPO_PASSWORD_SECRET_KEY]
		if !exist {
			// repo-password field not found, check if there is only one field
			if len(secret.Data) == 1 {
				// Use the only one field in the secret as password
				for _, password := range secret.Data {
					return string(password), true, nil
				}
			}
			return "", true, fmt.Errorf("%s secret should have '%s' field", repoPasswordSecretRef, chev1.RESTIC_REPO_PASSWORD_SECRET_KEY)
		}
		return string(password), true, nil
	} else if !errors.IsNotFound(err) {
		return "", false, err
	}
	return "", true, fmt.Errorf("secret '%s' with restic repository password not found", repoPasswordSecretRef)
}

// getPortString returns port part of the url: ':port' or empty string for default port
func getPortString(port int) string {
	if port != 0 {
		return ":" + strconv.Itoa(port)
	}
	return ""
}
