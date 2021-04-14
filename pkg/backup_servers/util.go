//
// Copyright (c) 2021 Red Hat, Inc.
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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getResticRepoPassword checks if the password for restic repository is specified and retrieves it.
// It doesn't check the password correctness.
// Returns:
//  - password or empty string if password is not set.
//  - done status
//  - error if any
// Password from CR takes precedence over the password from secret.
func getResticRepoPassword(client client.Client, namespace string, rp orgv1.RepoPassword) (string, bool, error) {
	if rp.Password != "" {
		return rp.Password, true, nil
	}

	if rp.PasswordSecretRef == "" {
		return "", true, fmt.Errorf("restic repository password should be specified")
	}
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{Namespace: namespace, Name: rp.PasswordSecretRef}
	err := client.Get(context.TODO(), namespacedName, secret)
	if err == nil {
		password, exist := secret.Data["repo-password"]
		if !exist {
			// repo-password field not found, check if there is only one field
			if len(secret.Data) == 1 {
				// Use the only one field in the secret as password
				for _, password := range secret.Data {
					return string(password), true, nil
				}
			}
			return "", true, fmt.Errorf("%s secret should have 'repo-password' field", rp.PasswordSecretRef)
		}
		return string(password), true, nil
	} else if !errors.IsNotFound(err) {
		return "", false, err
	}
	return "", true, fmt.Errorf("secret '%s' with restic repository password not found", rp.PasswordSecretRef)
}

// getPortString returns port part of the url: ':port' or empty string for default port
func getPortString(port int) string {
	if port != 0 {
		return ":" + strconv.Itoa(port)
	}
	return ""
}
