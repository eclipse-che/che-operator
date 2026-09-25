//
// Copyright (c) 2019-2026 Red Hat, Inc.
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

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	k8sclient "github.com/eclipse-che/che-operator/pkg/common/k8s-client"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

var SecretDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
}

// SyncSecretToCluster applies secret into cluster or external namespace
func SyncSecretToCluster(
	deployContext *chetypes.DeployContext,
	name string,
	data map[string][]byte) error {

	secretSpec := GetSecretSpec(name, deployContext.CheCluster.Namespace, data)

	if err := controllerutil.SetControllerReference(deployContext.CheCluster, secretSpec, deployContext.ClusterAPI.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for Secret %s/%s: %w", secretSpec.Namespace, name, err)
	}

	if err := deployContext.ClusterAPI.ClientWrapper.Sync(
		deployContext.Context,
		secretSpec,
		&k8sclient.SyncOptions{DiffOpts: SecretDiffOpts},
	); err != nil {
		return fmt.Errorf("failed to sync Secret %s/%s: %w", secretSpec.Namespace, name, err)
	}

	return nil
}

// Get all secrets by labels and annotations
func GetSecrets(deployContext *chetypes.DeployContext, labels map[string]string, annotations map[string]string) ([]corev1.Secret, error) {
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
func GetSecretSpec(name string, namespace string, data map[string][]byte) *corev1.Secret {
	labels := GetLabels(defaults.GetCheFlavor())
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
