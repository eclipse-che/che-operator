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

package tls

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/eclipse-che/che-operator/pkg/common/constants"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// IsSelfSignedCASecretExists checks if CheTLSSelfSignedCertificateSecretName exists so depending components can mount it
func IsSelfSignedCASecretExists(cheCtx *chetypes.CheContext) (bool, error) {
	cheTLSSelfSignedCertificateSecret := &corev1.Secret{}
	err := cheCtx.ClusterAPI.Client.Get(context.TODO(), types.NamespacedName{Namespace: cheCtx.CheCluster.Namespace, Name: constants.SelfSignedCertificateSecretName}, cheTLSSelfSignedCertificateSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetCheCABundles returns list of config maps with additional CA certificates that should be trusted by Che
// The selection is based on the specific label
func GetCheCABundles(client k8sclient.Client, namespace string) ([]corev1.ConfigMap, error) {
	CACertsConfigMapList := &corev1.ConfigMapList{}

	caBundleLabelSelectorRequirement, _ := labels.NewRequirement(constants.KubernetesComponentLabelKey, selection.Equals, []string{constants.CheCABundle})
	cheComponetLabelSelectorRequirement, _ := labels.NewRequirement(constants.KubernetesPartOfLabelKey, selection.Equals, []string{constants.CheEclipseOrg})
	listOptions := &k8sclient.ListOptions{
		LabelSelector: labels.NewSelector().Add(*cheComponetLabelSelectorRequirement).Add(*caBundleLabelSelectorRequirement),
		Namespace:     namespace,
	}
	if err := client.List(context.TODO(), CACertsConfigMapList, listOptions); err != nil {
		return nil, err
	}

	return CACertsConfigMapList.Items, nil
}

// GetAdditionalCACertsConfigMapVersion returns revision of merged additional CA certs config map
func GetAdditionalCACertsConfigMapVersion(cheCtx *chetypes.CheContext) string {
	trustStoreConfigMap := &corev1.ConfigMap{}
	exists, _ := cheCtx.ClusterAPI.ClientWrapper.GetIgnoreNotFound(
		cheCtx.Context,
		types.NamespacedName{Name: CheMergedCABundleCertsCMName, Namespace: cheCtx.CheCluster.Namespace},
		trustStoreConfigMap,
	)
	if exists {
		return trustStoreConfigMap.ResourceVersion
	}

	return ""
}
