//
// Copyright (c) 2012-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//
package deploy

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var secretDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
}

// SyncSecretToCluster applies secret into cluster
func SyncSecretToCluster(
	deployContext *DeployContext,
	name string,
	data map[string][]byte) (*corev1.Secret, error) {

	specSecret, err := GetSpecSecret(deployContext, name, data)
	if err != nil {
		return nil, err
	}

	clusterSecret, err := GetClusterSecret(specSecret.Name, specSecret.Namespace, deployContext.ClusterAPI)
	if err != nil {
		return nil, err
	}

	if clusterSecret == nil {
		logrus.Infof("Creating a new object: %s, name %s", specSecret.Kind, specSecret.Name)
		err := deployContext.ClusterAPI.Client.Create(context.TODO(), specSecret)
		return specSecret, err
	}

	diff := cmp.Diff(clusterSecret, specSecret, secretDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterSecret.Kind, clusterSecret.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := deployContext.ClusterAPI.Client.Delete(context.TODO(), clusterSecret)
		if err != nil {
			return nil, err
		}

		err = deployContext.ClusterAPI.Client.Create(context.TODO(), specSecret)
		if err != nil {
			return nil, err
		}
	}

	return clusterSecret, nil
}

// GetClusterSecret retrieves given secret from cluster
func GetClusterSecret(name string, namespace string, clusterAPI ClusterAPI) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
	err := clusterAPI.Client.Get(context.TODO(), namespacedName, secret)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return secret, nil
}

// GetSpecSecret return default secret config for given data
func GetSpecSecret(deployContext *DeployContext, name string, data map[string][]byte) (*corev1.Secret, error) {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: deployContext.CheCluster.Namespace,
			Labels:    labels,
		},
		Data: data,
	}

	err := controllerutil.SetControllerReference(deployContext.CheCluster, secret, deployContext.ClusterAPI.Scheme)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// CreateTLSSecretFromEndpoint creates TLS secret with given name which contains certificates obtained from the given url.
// If the url is empty string, then cluster default certificate will be obtained.
// Does nothing if secret with given name already exists.
func CreateTLSSecretFromEndpoint(deployContext *DeployContext, url string, name string) (err error) {
	secret := &corev1.Secret{}
	if err := deployContext.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: deployContext.CheCluster.Namespace}, secret); err != nil && errors.IsNotFound(err) {
		crtBytes, err := GetEndpointTLSCrtBytes(deployContext, url)
		if err != nil {
			logrus.Errorf("Failed to extract certificate for secret %s. Failed to create a secret with a self signed crt: %s", name, err)
			return err
		}

		secret, err = SyncSecretToCluster(deployContext, name, map[string][]byte{"ca.crt": crtBytes})
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteSecret - delete secret by name and namespace
func DeleteSecret(secretName string, namespace string, runtimeClient client.Client) error {
	logrus.Infof("Delete secret: %s in the namespace: %s", secretName, namespace)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}

	if err := runtimeClient.Delete(context.TODO(), secret); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

// CreateSecret - create secret by name and namespace
func CreateSecret(content map[string][]byte, secretName string, namespace string, runtimeClient client.Client) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: content,
	}
	if err := runtimeClient.Create(context.TODO(), secret); err != nil {
		return err
	}
	return nil
}
