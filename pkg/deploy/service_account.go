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
	"fmt"

	"github.com/eclipse-che/che-operator/pkg/common/chetypes"
	defaults "github.com/eclipse-che/che-operator/pkg/common/operator-defaults"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func SyncServiceAccountToCluster(cheCtx *chetypes.CheContext, name string) error {
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cheCtx.CheCluster.Namespace,
			Labels:    GetLabels(defaults.GetCheFlavor()),
		},
	}

	if err := controllerutil.SetControllerReference(cheCtx.CheCluster, sa, cheCtx.ClusterAPI.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference for ServiceAccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}

	if err := cheCtx.ClusterAPI.ClientWrapper.CreateIfNotExists(cheCtx.Context, sa); err != nil {
		return fmt.Errorf("failed to create ServiceAccount %s/%s: %w", sa.Namespace, sa.Name, err)
	}

	return nil
}
