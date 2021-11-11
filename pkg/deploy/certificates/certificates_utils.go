//
// Copyright (c) 2012-2021 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package certificates

import (
	"context"

	"github.com/eclipse-che/che-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetCACertsConfigMaps returns list of config maps with additional CA certificates that should be trusted by Che
// The selection is based on the specific label
func GetCACertsConfigMaps(client k8sclient.Client, namespace string) ([]corev1.ConfigMap, error) {
	CACertsConfigMapList := &corev1.ConfigMapList{}

	caBundleLabelSelectorRequirement, _ := labels.NewRequirement(deploy.KubernetesComponentLabelKey, selection.Equals, []string{CheCACertsConfigMapLabelValue})
	cheComponetLabelSelectorRequirement, _ := labels.NewRequirement(deploy.KubernetesPartOfLabelKey, selection.Equals, []string{deploy.CheEclipseOrg})
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
func GetAdditionalCACertsConfigMapVersion(ctx *deploy.DeployContext) string {
	trustStoreConfigMap := &corev1.ConfigMap{}
	exists, _ := deploy.GetNamespacedObject(ctx, CheAllCACertsConfigMapName, trustStoreConfigMap)
	if exists {
		return trustStoreConfigMap.ResourceVersion
	}

	return ""
}
