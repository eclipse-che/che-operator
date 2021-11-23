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
package deploy

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
)

var SecretDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
}

// SyncSecret applies secret into cluster or external namespace
func SyncSecretToCluster(
	deployContext *DeployContext,
	name string,
	namespace string,
	data map[string][]byte) (bool, error) {

	secretSpec := GetSecretSpec(deployContext, name, namespace, data)
	return Sync(deployContext, secretSpec, SecretDiffOpts)
}

// Get all secrets by labels and annotations
func GetSecrets(deployContext *DeployContext, labels map[string]string, annotations map[string]string) ([]corev1.Secret, error) {
	secrets := []corev1.Secret{}

	labelSelector := k8slabels.NewSelector()
	for k, v := range labels {
		req, err := k8slabels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return secrets, err
		}
		labelSelector = labelSelector.Add(*req)
	}

	listOptions := &client.ListOptions{
		Namespace:     deployContext.CheCluster.Namespace,
		LabelSelector: labelSelector,
	}
	secretList := &corev1.SecretList{}
	if err := deployContext.ClusterAPI.Client.List(context.TODO(), secretList, listOptions); err != nil {
		return secrets, err
	}

	for _, secret := range secretList.Items {
		annotationsOk := true
		for k, v := range annotations {
			_, annotationExists := secret.Annotations[k]
			if !annotationExists || secret.Annotations[k] != v {
				annotationsOk = false
				break
			}
		}

		if annotationsOk {
			secrets = append(secrets, secret)
		}
	}

	return secrets, nil
}

// GetSecretSpec return default secret config for given data
func GetSecretSpec(deployContext *DeployContext, name string, namespace string, data map[string][]byte) *corev1.Secret {
	labels := GetLabels(deployContext.CheCluster, DefaultCheFlavor(deployContext.CheCluster))
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	return secret
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

		_, err = SyncSecretToCluster(deployContext, name, deployContext.CheCluster.Namespace, map[string][]byte{"ca.crt": crtBytes})
		if err != nil {
			return err
		}
	}

	return nil
}
