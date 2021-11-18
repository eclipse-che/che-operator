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
	"net/http"
	"strings"

	chev1 "github.com/eclipse-che/che-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestServer implements BackupServer
type RestServer struct {
	config *chev1.RestServerConfig
	ResticClient
}

func (s *RestServer) PrepareConfiguration(client client.Client, namespace string) (bool, error) {
	s.ResticClient = ResticClient{}

	repoPassword, done, err := getResticRepoPassword(client, namespace, s.config.RepositoryPasswordSecretRef)
	if err != nil || !done {
		return done, err
	}
	s.RepoPassword = repoPassword

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

	repo := s.config.RepositoryPath
	if repo != "" && !strings.HasSuffix(repo, "/") {
		repo += "/"
	}

	// Check backup server credentials if any
	credentials := ""
	if s.config.CredentialsSecretRef != "" {
		// Use secret as REST server credentials source
		secret := &corev1.Secret{}
		namespacedName := types.NamespacedName{Namespace: namespace, Name: s.config.CredentialsSecretRef}
		err = client.Get(context.TODO(), namespacedName, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, fmt.Errorf("secret '%s' with REST server username and password not found", s.config.CredentialsSecretRef)
			}
			return false, err
		}

		// Check the secret fields
		if value, exist := secret.Data[chev1.USERNAME_SECRET_KEY]; !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have '%s' field", secret.ObjectMeta.Name, chev1.USERNAME_SECRET_KEY)
		}
		if value, exist := secret.Data[chev1.PASSWORD_SECRET_KEY]; !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have '%s' field", secret.ObjectMeta.Name, chev1.PASSWORD_SECRET_KEY)
		}

		username := string(secret.Data[chev1.USERNAME_SECRET_KEY])
		password := string(secret.Data[chev1.PASSWORD_SECRET_KEY])
		if username != "" && password != "" {
			credentials = username + ":" + password + "@"
		}
	}

	// rest:https://user:password@host:5000/repo/
	s.RepoUrl = "rest:" + protocol + "://" + credentials + host + port + "/" + repo

	return true, nil
}

func (s *RestServer) IsRepositoryExist() (bool, bool, error) {
	defaultCheBackupRepoUrl := strings.TrimPrefix(s.ResticClient.RepoUrl, "rest:") + "config"
	response, err := http.Head(defaultCheBackupRepoUrl)
	if err != nil {
		return false, true, err
	}
	if response.StatusCode == 404 || response.ContentLength == 0 {
		// Cannot read the repository, probably it doesn't exist
		return false, true, nil
	}

	return true, true, nil
}
