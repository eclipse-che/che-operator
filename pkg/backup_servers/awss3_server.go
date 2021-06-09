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

	orgv1 "github.com/eclipse-che/che-operator/pkg/apis/org/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RestServer implements BackupServer
type AwsS3Server struct {
	config       orgv1.AwsS3ServerConfig
	repoPassword string
	url          string
	secretKeyId  string
	secretKey    string
}

func (s *AwsS3Server) PrepareConfiguration(client client.Client, namespace string) (bool, error) {
	repoPassword, done, err := getResticRepoPassword(client, namespace, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}
	s.repoPassword = repoPassword

	repo := s.config.Repo
	if repo == "" {
		return true, fmt.Errorf("bucket (repository) must be configured")
	}

	protocol := s.config.Protocol
	if protocol != "" {
		protocol += "://"
	}
	host := s.config.Hostname
	if host == "" {
		host = "s3.amazonaws.com"
	}
	port := getPortString(s.config.Port)

	// Ensure access key and its ID provided
	secretKeyId := s.config.AwsAccessKeyId
	secretKey := s.config.AwsSecretAccessKey
	if secretKeyId == "" || secretKey == "" {
		// Read key from the secret
		if s.config.AwsAccessKeySecretRef == "" {
			return true, fmt.Errorf("secret with access key is not provided")
		}

		secret := &corev1.Secret{}
		namespacedName := types.NamespacedName{Namespace: namespace, Name: s.config.AwsAccessKeySecretRef}
		err = client.Get(context.TODO(), namespacedName, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				return true, fmt.Errorf("secret '%s' with access key not found", s.config.AwsAccessKeySecretRef)
			}
			return false, err
		}

		// Check the secret fields
		value, exist := secret.Data["awsAccessKeyId"]
		if !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have access key ID under 'awsAccessKeyId' field", secret.ObjectMeta.Name)
		}
		secretKeyId = string(value)

		value, exist = secret.Data["awsSecretAccessKey"]
		if !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have access key under 'awsSecretAccessKey' field", secret.ObjectMeta.Name)
		}
		secretKey = string(value)
	}
	s.secretKeyId = secretKeyId
	s.secretKey = secretKey

	// s3:s3.amazonaws.com/bucket
	// s3:http://server:port/bucket/repo
	s.url = "s3:" + protocol + host + port + "/" + repo

	return true, nil
}
