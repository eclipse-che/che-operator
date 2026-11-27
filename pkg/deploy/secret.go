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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

var SecretDiffOpts = cmp.Options{
	cmpopts.IgnoreFields(corev1.Secret{}, "TypeMeta", "ObjectMeta"),
}

// Get all secrets by labels and annotations
func GetSecrets(cheCtx *chetypes.CheContext, labels map[string]string, annotations map[string]string) ([]corev1.Secret, error) {
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
		Namespace:     cheCtx.CheCluster.Namespace,
		LabelSelector: labelSelector,
	}
	secretList := &corev1.SecretList{}
	if err := cheCtx.ClusterAPI.Client.List(context.TODO(), secretList, listOptions); err != nil {
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
