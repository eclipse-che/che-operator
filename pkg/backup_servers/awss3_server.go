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

const (
	awsAccesKeyIdEnvVarName = "AWS_ACCESS_KEY_ID"
	awsAccesKeyEnvVarName   = "AWS_SECRET_ACCESS_KEY"
)

// AwsS3Server implements BackupServer
type AwsS3Server struct {
	config orgv1.AwsS3ServerConfig
	ResticClient
	secretKeyId string
	secretKey   string
}

func (s *AwsS3Server) PrepareConfiguration(client client.Client, namespace string) (bool, error) {
	s.ResticClient = ResticClient{}

	repoPassword, done, err := getResticRepoPassword(client, namespace, s.config.RepoPassword)
	if err != nil || !done {
		return done, err
	}
	s.RepoPassword = repoPassword

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
		value, exist := secret.Data[orgv1.AWS_ACCESS_KEY_ID_SECRET_KEY]
		if !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have access key ID under '%s' field", secret.ObjectMeta.Name, orgv1.AWS_ACCESS_KEY_ID_SECRET_KEY)
		}
		secretKeyId = string(value)

		value, exist = secret.Data[orgv1.AWS_SECRET_ACCESS_KEY_SECRET_KEY]
		if !exist || string(value) == "" {
			return true, fmt.Errorf("%s secret should have access key under '%s' field", secret.ObjectMeta.Name, orgv1.AWS_SECRET_ACCESS_KEY_SECRET_KEY)
		}
		secretKey = string(value)
	}
	s.secretKeyId = secretKeyId
	s.secretKey = secretKey

	// s3:s3.amazonaws.com/bucket
	// s3:http://server:port/bucket/repo
	s.RepoUrl = "s3:" + protocol + host + port + "/" + repo

	// Configure required env variables
	s.AdditionalEnv = s.getAdditionalEnv()

	return true, nil
}

func (s *AwsS3Server) getAdditionalEnv() []string {
	return []string{
		awsAccesKeyIdEnvVarName + "=" + s.secretKeyId,
		awsAccesKeyEnvVarName + "=" + s.secretKey,
	}
}
