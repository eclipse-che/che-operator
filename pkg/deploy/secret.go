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

	orgv1 "github.com/eclipse/che-operator/pkg/apis/org/v1"
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
	checluster *orgv1.CheCluster,
	name string,
	data map[string][]byte,
	clusterAPI ClusterAPI) (*corev1.Secret, error) {

	specSecret := GetSpecSecret(checluster, name, data)

	clusterSecret, err := GetClusterSecret(specSecret.Name, specSecret.Namespace, clusterAPI)
	if err != nil {
		return nil, err
	}

	if clusterSecret == nil {
		logrus.Infof("Creating a new object: %s, name %s", specSecret.Kind, specSecret.Name)
		err := clusterAPI.Client.Create(context.TODO(), specSecret)
		return specSecret, err
	}

	diff := cmp.Diff(clusterSecret, specSecret, secretDiffOpts)
	if len(diff) > 0 {
		logrus.Infof("Updating existed object: %s, name: %s", clusterSecret.Kind, clusterSecret.Name)
		fmt.Printf("Difference:\n%s", diff)

		err := clusterAPI.Client.Delete(context.TODO(), clusterSecret)
		if err != nil {
			return nil, err
		}

		err = clusterAPI.Client.Create(context.TODO(), specSecret)
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
func GetSpecSecret(cr *orgv1.CheCluster, name string, data map[string][]byte) *corev1.Secret {
	labels := GetLabels(cr, DefaultCheFlavor(cr))
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

// CreateTLSSecretFromRoute creates TLS secret with given name which contains certificates obtained from give url.
// If the url is empty string, then router certificate will be obtained.
// Works only on Openshift family infrastructures.
func CreateTLSSecretFromRoute(checluster *orgv1.CheCluster, url string, name string, clusterAPI ClusterAPI) (err error) {
	secret := &corev1.Secret{}
	if err := clusterAPI.Client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: checluster.Namespace}, secret); err != nil && errors.IsNotFound(err) {
		crtBytes, err := GetEndpointTLSCrtBytes(checluster, url, clusterAPI)
		if err != nil {
			logrus.Errorf("Failed to extract certificate for secret %s. Failed to create a secret with a self signed crt: %s", name, err)
			return err
		}

		secret, err = SyncSecretToCluster(checluster, name, map[string][]byte{"ca.crt": crtBytes}, clusterAPI)
		if err != nil {
			return err
		}
	}

	return nil
}
