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
	"strings"

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestServer implements BackupServer
type RestServer struct {
	config       orgv1.RestServerConfing
	repoPassword string
	url          string
}

func (s *RestServer) PrepareConfiguration(client client.Client, namespace string) (bool, error) {
	repoPassword, done, err := getResticRepoPassword(client, namespace, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}
	s.repoPassword = repoPassword

	protocol := s.config.Protocol
	if protocol == "" {
		protocol = "https"
	}
	if !(protocol == "http" || protocol == "https") {
		return true, fmt.Errorf("unrecognized protocol %s for REST server", protocol)
	}

	host := s.config.Hostname
	if host == "" {
		return true, fmt.Errorf("REST server hostname must be configured")
	}
	port := getPortString(s.config.Port)

	repo := s.config.Repo
	if repo != "" && !strings.HasSuffix(repo, "/") {
		repo += "/"
	}

	// Check backup server credentials if any
	username := s.config.Username
	password := s.config.Password
	if (username == "" || password == "") && s.config.CredentialsSecretRef != "" {
		// Use secret as credentials source
		secret := &corev1.Secret{}
		namespacedName := types.NamespacedName{Namespace: namespace, Name: s.config.CredentialsSecretRef}
		err = client.Get(context.TODO(), namespacedName, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, fmt.Errorf("secret '%s' with username and password not found", s.config.CredentialsSecretRef)
			}
			return false, err
		}

		// Check the secret fields
		if value, exist := secret.Data["username"]; !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have 'username' field", secret.ObjectMeta.Name)
		}
		if value, exist := secret.Data["password"]; !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have 'password' field", secret.ObjectMeta.Name)
		}

		username = string(secret.Data["username"])
		password = string(secret.Data["password"])
	}
	credentials := ""
	if username != "" && password != "" {
		credentials = username + ":" + password + "@"
	}

	// rest:https://user:password@host:5000/repo/
	s.url = "rest:" + protocol + "://" + credentials + host + port + "/" + repo

	return true, nil
}
