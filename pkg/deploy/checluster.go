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

	chev2 "github.com/eclipse-che/che-operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
)

func UpdateCheCRStatus(cheCtx *chetypes.CheContext, field string, value string) (err error) {
	err = cheCtx.ClusterAPI.Client.Status().Update(context.TODO(), cheCtx.CheCluster)
	if err == nil {
		logrus.Infof("Custom resource status %s updated with %s: %s", cheCtx.CheCluster.Name, field, value)
		return nil
	}

	return err
}

func SetStatusDetails(cheCtx *chetypes.CheContext, reason string, message string) (err error) {
	if reason != cheCtx.CheCluster.Status.Reason {
		cheCtx.CheCluster.Status.Reason = reason
		if err := UpdateCheCRStatus(cheCtx, "status: Reason", reason); err != nil {
			return err
		}
	}
	if message != cheCtx.CheCluster.Status.Message {
		cheCtx.CheCluster.Status.Message = message
		if err := UpdateCheCRStatus(cheCtx, "status: Message", message); err != nil {
			return err
		}
	}
	return nil
}

func ReloadCheClusterCR(cheCtx *chetypes.CheContext) error {
	cheCluster := &chev2.CheCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CheCluster",
			APIVersion: chev2.GroupVersion.String(),
		},
	}

	if err := cheCtx.ClusterAPI.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: cheCtx.CheCluster.Name, Namespace: cheCtx.CheCluster.Namespace},
		cheCluster); err != nil {
		return err
	}

	cheCtx.CheCluster = cheCluster
	return nil
}

// FindCheClusterCRInNamespace returns CheCluster custom resource from given namespace.
// If namespace is empty then checluster will be found in any namespace.
// Only one instance of CheCluster custom resource is expected.
func FindCheClusterCRInNamespace(cl client.Client, namespace string) (*chev2.CheCluster, error) {
	cheClusters := &chev2.CheClusterList{}
	if err := cl.List(context.TODO(), cheClusters, &client.ListOptions{Namespace: namespace}); err != nil {
		return nil, err
	}

	num := len(cheClusters.Items)
	switch num {
	case 0:
		return nil, nil
	case 1:
		return &cheClusters.Items[0], nil
	default:
		return nil, fmt.Errorf("expected one instance of CheCluster custom resources, but '%d' found", len(cheClusters.Items))
	}
}
