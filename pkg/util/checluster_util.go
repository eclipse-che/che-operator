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
package util

import (
	"context"
	"fmt"

	orgv1 "github.com/eclipse-che/che-operator/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Finds checluster custom resource in a given namespace.
// If namespace is empty then checluster will be found in any namespace.
func FindCheClusterCRInNamespace(cl client.Client, namespace string) (*orgv1.CheCluster, int, error) {
	cheClusters := &orgv1.CheClusterList{}
	listOptions := &client.ListOptions{Namespace: namespace}
	if err := cl.List(context.TODO(), cheClusters, listOptions); err != nil {
		return nil, -1, err
	}

	if len(cheClusters.Items) != 1 {
		return nil, len(cheClusters.Items), fmt.Errorf("Expected one instance of CheCluster custom resources, but '%d' found.", len(cheClusters.Items))
	}

	checluster := &orgv1.CheCluster{}
	namespacedName := types.NamespacedName{Namespace: cheClusters.Items[0].GetNamespace(), Name: cheClusters.Items[0].GetName()}
	err := cl.Get(context.TODO(), namespacedName, checluster)
	if err != nil {
		return nil, -1, err
	}
	return checluster, 1, nil
}
